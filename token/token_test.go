package token

import (
	"testing"
	"time"
)

func TestTokenizationWorks(t *testing.T) {

	key := []byte("keys are 16 byte")
	j := &JobToken{"test", "124", time.Now()}

	token, err := Tokenize(key, j)
	if err != nil {
		t.Error(err)
	}

	j2, err2 := Untokenize(key, token)

	if err2 != nil {
		t.Error(err2)
	}

	if !j.Equal(j2) {
		t.Error("Tokenization integrity failed")
	}
}

func TestTokenizationFailsHMAC(t *testing.T) {
	key := []byte("keys are 16 byte")
	j := &JobToken{"test", "124", time.Now()}

	token, _ := Tokenize(key, j)

	_t := []byte(token)
	_t[2] = _t[2] + 1 // just change a bit somewhere

	token = string(_t)

	_, err := Untokenize(key, token)

	if err == nil {
		t.Error("error expected")
	}
}
