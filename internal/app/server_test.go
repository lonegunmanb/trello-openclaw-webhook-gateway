package app

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type capturedRequest struct {
	AuthHeader  string
	ContentType string
	Body        []byte
}

func signTrello(secret string, rawBody []byte, callbackURL string) string {
	h := hmac.New(sha1.New, []byte(secret))
	_, _ = h.Write(rawBody)
	_, _ = h.Write([]byte(callbackURL))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func payload(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func setupForwardServer(t *testing.T, status int, responseBody string) (*httptest.Server, chan capturedRequest) {
	t.Helper()
	ch := make(chan capturedRequest, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		ch <- capturedRequest{AuthHeader: r.Header.Get("Authorization"), ContentType: r.Header.Get("Content-Type"), Body: body}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(responseBody))
	}))
	return srv, ch
}

func TestHeadReturns200(t *testing.T) {
	forwardSrv, _ := setupForwardServer(t, http.StatusOK, "ok")
	defer forwardSrv.Close()

	cfg := Config{ListenAddr: ":0", TrelloSecret: "secret", CallbackURL: "https://example.com/trello", ForwardURL: forwardSrv.URL, ForwardToken: "token"}
	r := NewRouter(cfg, &http.Client{Timeout: time.Second}, log.Default())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestPostRejectsBadSignature403(t *testing.T) {
	forwardSrv, ch := setupForwardServer(t, http.StatusOK, "ok")
	defer forwardSrv.Close()

	cfg := Config{ListenAddr: ":0", TrelloSecret: "secret", CallbackURL: "https://example.com/trello", ForwardURL: forwardSrv.URL, ForwardToken: "token"}
	r := NewRouter(cfg, &http.Client{Timeout: time.Second}, log.Default())

	body := payload(t, map[string]any{"action": map[string]any{"type": "updateCard"}})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-Trello-Webhook", "bad-sign")
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	select {
	case <-ch:
		t.Fatal("should not forward request")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPostValidSignatureForwards(t *testing.T) {
	forwardSrv, ch := setupForwardServer(t, http.StatusAccepted, "accepted")
	defer forwardSrv.Close()

	cfg := Config{ListenAddr: ":0", TrelloSecret: "secret", CallbackURL: "https://example.com/trello", ForwardURL: forwardSrv.URL, ForwardToken: "fwd-token"}
	r := NewRouter(cfg, &http.Client{Timeout: time.Second}, log.Default())

	body := payload(t, map[string]any{
		"action": map[string]any{
			"type": "updateCard",
			"data": map[string]any{
				"card":       map[string]any{"name": "Fix bug"},
				"board":      map[string]any{"name": "Main Board"},
				"listBefore": map[string]any{"name": "Ready for review"},
				"listAfter":  map[string]any{"name": "Approved for action"},
			},
			"memberCreator": map[string]any{"fullName": "Roger"},
		},
	})
	sig := signTrello(cfg.TrelloSecret, body, cfg.CallbackURL)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-Trello-Webhook", sig)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected proxied status 202, got %d", rr.Code)
	}

	var got capturedRequest
	select {
	case got = <-ch:
	case <-time.After(time.Second):
		t.Fatal("expected forwarded request")
	}
	if got.AuthHeader != "Bearer fwd-token" {
		t.Fatalf("unexpected auth header: %q", got.AuthHeader)
	}

	var raw map[string]any
	if err := json.Unmarshal(got.Body, &raw); err != nil {
		t.Fatalf("unmarshal forwarded body: %v", err)
	}
	action, _ := raw["action"].(map[string]any)
	if action == nil || action["type"] != "updateCard" {
		t.Fatalf("forwarded body missing action.type: %v", raw)
	}
}

func TestPostPropagatesDownstreamStatus(t *testing.T) {
	forwardSrv, _ := setupForwardServer(t, http.StatusNoContent, "")
	defer forwardSrv.Close()

	cfg := Config{ListenAddr: ":0", TrelloSecret: "secret", CallbackURL: "https://example.com/trello", ForwardURL: forwardSrv.URL, ForwardToken: "token"}
	r := NewRouter(cfg, &http.Client{Timeout: time.Second}, log.Default())

	body := payload(t, map[string]any{"action": map[string]any{"type": "updateCard"}})
	sig := signTrello(cfg.TrelloSecret, body, cfg.CallbackURL)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-Trello-Webhook", sig)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	forwardSrv, _ := setupForwardServer(t, http.StatusOK, "ok")
	defer forwardSrv.Close()

	cfg := Config{ListenAddr: ":0", TrelloSecret: "secret", CallbackURL: "https://example.com/trello", ForwardURL: forwardSrv.URL, ForwardToken: "token"}
	r := NewRouter(cfg, &http.Client{Timeout: time.Second}, log.Default())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
