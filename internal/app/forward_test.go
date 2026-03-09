package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestForwardSetsAuthorizationHeader(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	_, _, err := f.Forward(context.Background(), "hi")
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if auth != "Bearer token" {
		t.Fatalf("unexpected auth header: %q", auth)
	}
}

func TestForwardContentTypeJSON(t *testing.T) {
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	_, _, err := f.Forward(context.Background(), "hi")
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("unexpected content-type: %q", contentType)
	}
}

func TestForwardBodyShape(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	_, _, err := f.Forward(context.Background(), "hello")
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	var v map[string]any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v["message"] != "hello" || v["name"] != "Trello" || v["deliver"] != true || v["channel"] != "telegram" || v["to"] != "399076135" {
		t.Fatalf("unexpected payload: %v", v)
	}
}

func TestForwardPropagatesResponseStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	status, respBody, err := f.Forward(context.Background(), "hello")
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if status != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", status)
	}
	if string(respBody) != "accepted" {
		t.Fatalf("unexpected response body: %s", string(respBody))
	}
}

func TestForwardTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: 10 * time.Millisecond})
	_, _, err := f.Forward(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
