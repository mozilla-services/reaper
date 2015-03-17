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
)

const (
	DELIMIT = "|"
)

type JobToken struct {
	Action     string
	InstanceId string
	ValidUntil time.Time
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

func Encrypt(key []byte, j *JobToken) ([]byte, error) {

	jsonData := j.JSON()

	block, berr := aes.NewCipher(key)
	if berr != nil {
		return nil, berr
	}

	ciphertext := make([]byte, aes.BlockSize+len(jsonData))

	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], jsonData)
	return ciphertext, nil
}

func Decrypt(key, ciphertext []byte) ([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

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

func Tokenize(key []byte, j *JobToken) (string, error) {

	ciphertext, err := Encrypt(key, j)
	if err != nil {
		return "", err
	}

	hmac := HMAC(key, ciphertext)
	return (base64.URLEncoding.EncodeToString(ciphertext) +
		DELIMIT + base64.URLEncoding.EncodeToString(hmac)), nil
}

func Untokenize(key []byte, t string) (j *JobToken, err error) {

	var ciphertext, hmacVerify, jsonData []byte

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
