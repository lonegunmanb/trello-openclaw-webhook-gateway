package app

import (
	"context"
	"encoding/base64"
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
	_, _, err := f.Forward(context.Background(), "hi", []byte(`{"k":"v"}`))
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
	_, _, err := f.Forward(context.Background(), "hi", []byte(`{"k":"v"}`))
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
	_, _, err := f.Forward(context.Background(), "hello", []byte(`{"k":"v"}`))
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	var v map[string]any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v["name"] != "Trello" || v["deliver"] != false {
		t.Fatalf("unexpected payload: %v", v)
	}
	if _, ok := v["channel"]; ok {
		t.Fatalf("channel should not be present: %v", v)
	}
	if _, ok := v["to"]; ok {
		t.Fatalf("to should not be present: %v", v)
	}
	msg, ok := v["message"].(string)
	if !ok {
		t.Fatalf("message must be string, got %T", v["message"])
	}
	decodedRaw, err := base64.StdEncoding.DecodeString(msg)
	if err != nil {
		t.Fatalf("decode raw payload: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(decodedRaw, &raw); err != nil {
		t.Fatalf("unmarshal decoded raw payload: %v", err)
	}
	if raw["k"] != "v" {
		t.Fatalf("decoded raw payload missing original field: %v", raw)
	}
	if raw["readable_message"] != "hello" {
		t.Fatalf("decoded raw payload missing readable_message: %v", raw)
	}
	if _, ok := v["model"]; ok {
		t.Fatalf("model should not be present: %v", v)
	}
}

func TestForwardPropagatesResponseStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	status, respBody, err := f.Forward(context.Background(), "hello", []byte(`{"k":"v"}`))
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
	_, _, err := f.Forward(context.Background(), "hello", []byte(`{"k":"v"}`))
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
