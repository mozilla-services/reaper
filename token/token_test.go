package token

import (
	"crypto/hmac"
	"encoding/base64"
	"testing"
)

var (
	t_secret = []byte("testing secret")
	t_salt   = []byte("testing salt")
	t_vars   = map[string]string{"a": "1", "b": "2"}
)

func TestCreateToken(t *testing.T) {

	t1 := CreateToken(t_secret, t_salt, t_vars)
	b := base64.URLEncoding.EncodeToString(t1)
	if b != "kzrkSs9Wroq-kbYg0otFoi4ddQ95PdXJXJ0syA-HPUU=" {
		t.Error("Unexpected Token", b)
	}
}

func TestPayloadKeyValueSorting(t *testing.T) {

	v := map[string]string{"b": "2", "a": "1"}

	t1 := CreateToken(t_secret, t_salt, t_vars)
	t2 := CreateToken(t_secret, t_salt, v)

	if !hmac.Equal(t1, t2) {
		t.Error("payload creation failed")
	}

}
