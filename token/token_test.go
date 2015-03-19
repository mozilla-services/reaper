package token

import (
	"testing"
	"time"
)

const (
	t_password = "my super strong password"
)

func TestTokenizationWorks(t *testing.T) {

	j := &JobToken{"test", "124", time.Now()}

	token, err := Tokenize(t_password, j)
	if err != nil {
		t.Error(err)
	}

	j2, err2 := Untokenize(t_password, token)

	if err2 != nil {
		t.Error(err2)
	}

	if !j.Equal(j2) {
		t.Error("Tokenization integrity failed")
	}
}

func TestTokenizationFailsHMAC(t *testing.T) {
	j := &JobToken{"test", "124", time.Now()}

	token, _ := Tokenize(t_password, j)

	_t := []byte(token)
	_t[2] = _t[2] + 1 // just change a bit somewhere

	token = string(_t)

	_, err := Untokenize(t_password, token)

	if err == nil {
		t.Error("error expected")
	}
}
