package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"math/rand"
	"sort"
	"time"

	"golang.org/x/crypto/scrypt"
)

const (
	SALT_SIZE  = 16
	SCRYPT_N   = 16384
	SCRYPT_R   = 8
	SCRYPT_P   = 2
	SCRYPT_LEN = 32
)

func init() {
	// makes sures we are always seeded
	rand.Seed(time.Now().UTC().UnixNano())
}

func createPayload(m map[string]string) []string {
	// add values to the hmac in sorted order
	var keys []string
	var vals []string
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		vals = append(vals, k+":"+m[k])
	}

	return vals
}

func CreateToken(secret, salt []byte, vars map[string]string) []byte {
	sKey, _ := scrypt.Key(secret, salt, SCRYPT_N, SCRYPT_R, SCRYPT_P, SCRYPT_LEN)
	mac := hmac.New(sha256.New, sKey)
	mac.Write(salt)

	for _, v := range createPayload(vars) {
		mac.Write([]byte(v))

	}

	return mac.Sum(nil)
}
