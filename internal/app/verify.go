package app

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
)

func VerifySignature(secret string, rawBody []byte, callbackURL, headerValue string) bool {
	if headerValue == "" {
		return false
	}

	h := hmac.New(sha1.New, []byte(secret))
	_, _ = h.Write(rawBody)
	_, _ = h.Write([]byte(callbackURL))
	expected := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(headerValue))
}
