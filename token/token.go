package token

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"
)

const (
	tag_delimiter = "|"
	scrypt_n      = 16384
	scrypt_p      = 1
	scrypt_r      = 8
	scrypt_keyLen = 32

	tokenDuration = time.Duration(8 * 24 * time.Hour)
)

//go:generate stringer -type=Type
type Type int

const (
	J_DELAY Type = iota
	J_TERMINATE
	J_WHITELIST
	J_STOP
	J_FORCESTOP
)

// Not very scalable but good enough for our requirements
type JobToken struct {
	Action      Type
	ID          string
	Region      string
	IgnoreUntil time.Duration
	ValIDUntil  time.Time
}

func (j *JobToken) JSON() []byte {
	b, _ := json.Marshal(j)
	return b
}

func (j *JobToken) Equal(j2 *JobToken) bool {

	return j.Action != j2.Action ||
		j.ID != j2.ID ||
		j.ValIDUntil.Equal(j2.ValIDUntil)
}

func (j *JobToken) Expired() bool {
	return j.ValIDUntil.Before(time.Now())
}

func NewDelayJob(region, ID string, until time.Duration) *JobToken {
	return &JobToken{
		Action:      J_DELAY,
		ID:          ID,
		Region:      region,
		IgnoreUntil: until,
		ValIDUntil:  time.Now().Add(tokenDuration),
	}
}

func NewTerminateJob(region, ID string) *JobToken {
	return &JobToken{
		Action:     J_TERMINATE,
		ID:         ID,
		Region:     region,
		ValIDUntil: time.Now().Add(tokenDuration),
	}
}

func NewWhitelistJob(region, ID string) *JobToken {
	return &JobToken{
		Action:     J_WHITELIST,
		ID:         ID,
		Region:     region,
		ValIDUntil: time.Now().Add(tokenDuration),
	}
}

func NewStopJob(region, ID string) *JobToken {
	return &JobToken{
		Action:     J_STOP,
		ID:         ID,
		Region:     region,
		ValIDUntil: time.Now().Add(tokenDuration),
	}
}

func NewForceStopJob(region, ID string) *JobToken {
	return &JobToken{
		Action:     J_FORCESTOP,
		ID:         ID,
		Region:     region,
		ValIDUntil: time.Now().Add(tokenDuration),
	}
}

func encryptToken(key []byte, j *JobToken) ([]byte, error) {

	jsonData := j.JSON()

	ciphertext := make([]byte, aes.BlockSize+len(jsonData))

	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	sKey, serr := scrypt.Key(key, iv, scrypt_n, scrypt_r, scrypt_p, scrypt_keyLen)

	if serr != nil {
		return nil, serr
	}

	block, berr := aes.NewCipher(sKey)
	if berr != nil {
		return nil, berr
	}

	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], jsonData)
	return ciphertext, nil
}

func decryptToken(key, ciphertext []byte) ([]byte, error) {

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	sKey, serr := scrypt.Key(key, iv, scrypt_n, scrypt_r, scrypt_p, scrypt_keyLen)

	if serr != nil {
		return nil, serr
	}
	block, err := aes.NewCipher(sKey)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}

func makeHMAC(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func Tokenize(password string, j *JobToken) (string, error) {
	key := []byte(password)

	ciphertext, err := encryptToken(key, j)
	if err != nil {
		return "", err
	}

	hmac := makeHMAC(key, ciphertext)
	return (base64.URLEncoding.EncodeToString(ciphertext) +
		tag_delimiter + base64.URLEncoding.EncodeToString(hmac)), nil
}

func Untokenize(password, t string) (j *JobToken, err error) {

	var ciphertext, hmacVerify, jsonData []byte
	key := []byte(password)

	parts := strings.Split(t, tag_delimiter)

	if len(parts) != 2 {
		return nil, errors.New("InvalID token")
	}

	ciphertext, err = base64.URLEncoding.DecodeString(parts[0])

	if err != nil {
		return
	}

	hmacVerify, err = base64.URLEncoding.DecodeString(parts[1])

	if err != nil {
		return
	}

	if !hmac.Equal(hmacVerify, makeHMAC(key, ciphertext)) {
		return nil, errors.New("HMAC check failed")
	}

	jsonData, err = decryptToken(key, ciphertext)

	if err != nil {
		return
	}

	err = json.Unmarshal(jsonData, &j)

	if err != nil {
		return
	}

	return
}
