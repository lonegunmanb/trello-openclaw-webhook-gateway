package app

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"testing"
)

func sign(secret string, rawBody []byte, callbackURL string) string {
	h := hmac.New(sha1.New, []byte(secret))
	_, _ = h.Write(rawBody)
	_, _ = h.Write([]byte(callbackURL))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func TestVerifySignatureValid(t *testing.T) {
	raw := []byte(`{"hello":"world"}`)
	secret := "s3cr3t"
	callbackURL := "https://example.com/trello"
	header := sign(secret, raw, callbackURL)

	if !VerifySignature(secret, raw, callbackURL, header) {
		t.Fatal("expected valid signature")
	}
}

func TestVerifySignatureInvalid(t *testing.T) {
	raw := []byte(`{"hello":"world"}`)
	if VerifySignature("secret", raw, "https://example.com/trello", "bad-signature") {
		t.Fatal("expected invalid signature")
	}
}

func TestVerifySignatureMissingHeader(t *testing.T) {
	raw := []byte(`{"hello":"world"}`)
	if VerifySignature("secret", raw, "https://example.com/trello", "") {
		t.Fatal("expected invalid signature when header missing")
	}
}
