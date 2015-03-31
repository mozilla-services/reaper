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
	DELIMIT       = "|"
	SCRYPT_N      = 16384
	SCRYPT_p      = 1
	SCRYPT_r      = 8
	SCRYPT_keyLen = 32

	TokenDuration = time.Duration(8 * 24 * time.Hour)
)

//go:generate stringer -type=Type
type Type int

const (
	J_DELAY Type = iota
	J_TERMINATE
)

// Not very scalable but good enough for our requirements
type JobToken struct {
	Action      Type
	InstanceId  string
	Region      string
	IgnoreUntil time.Time
	ValidUntil  time.Time
}

func NewDelayJob(region, instanceId string, until time.Time) *JobToken {
	return &JobToken{
		Action:      J_DELAY,
		InstanceId:  instanceId,
		Region:      region,
		IgnoreUntil: until,
		ValidUntil:  time.Now().Add(TokenDuration),
	}
}

func NewTerminateJob(region, instanceId string) *JobToken {
	return &JobToken{
		Action:     J_TERMINATE,
		InstanceId: instanceId,
		Region:     region,
		ValidUntil: time.Now().Add(TokenDuration),
	}
}

func (j *JobToken) JSON() []byte {
	b, _ := json.Marshal(j)
	return b
}

func (j *JobToken) Equal(j2 *JobToken) bool {

	return j.Action != j2.Action ||
		j.InstanceId != j2.InstanceId ||
		j.ValidUntil.Equal(j2.ValidUntil)
}

func (j *JobToken) Expired() bool {
	return j.ValidUntil.Before(time.Now())
}

func Encrypt(key []byte, j *JobToken) ([]byte, error) {

	jsonData := j.JSON()

	ciphertext := make([]byte, aes.BlockSize+len(jsonData))

	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	sKey, serr := scrypt.Key(key, iv, SCRYPT_N, SCRYPT_r, SCRYPT_p, SCRYPT_keyLen)

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

func Decrypt(key, ciphertext []byte) ([]byte, error) {

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	sKey, serr := scrypt.Key(key, iv, SCRYPT_N, SCRYPT_r, SCRYPT_p, SCRYPT_keyLen)

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

func HMAC(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func Tokenize(password string, j *JobToken) (string, error) {
	key := []byte(password)

	ciphertext, err := Encrypt(key, j)
	if err != nil {
		return "", err
	}

	hmac := HMAC(key, ciphertext)
	return (base64.URLEncoding.EncodeToString(ciphertext) +
		DELIMIT + base64.URLEncoding.EncodeToString(hmac)), nil
}

func Untokenize(password, t string) (j *JobToken, err error) {

	var ciphertext, hmacVerify, jsonData []byte
	key := []byte(password)

	parts := strings.Split(t, DELIMIT)

	if len(parts) != 2 {
		return nil, errors.New("Invalid token")
	}

	ciphertext, err = base64.URLEncoding.DecodeString(parts[0])

	if err != nil {
		return
	}

	hmacVerify, err = base64.URLEncoding.DecodeString(parts[1])

	if err != nil {
		return
	}

	if !hmac.Equal(hmacVerify, HMAC(key, ciphertext)) {
		return nil, errors.New("HMAC check failed")
	}

	jsonData, err = Decrypt(key, ciphertext)

	if err != nil {
		return
	}

	err = json.Unmarshal(jsonData, &j)

	if err != nil {
		return
	}

	return
}
