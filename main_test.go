package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/lonegunmanb/jjc/internal/app/localboard"
	"github.com/lonegunmanb/jjc/internal/app/sysevent"
)

func TestNewLocalBoardHTTPServerLeavesLongLivedTimeoutsDisabled(t *testing.T) {
	srv := newLocalBoardHTTPServer("127.0.0.1:0", http.NewServeMux())
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 || srv.IdleTimeout != 0 {
		t.Fatalf("local board server timeouts = read:%s write:%s idle:%s, want disabled for SSE",
			srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}
}

func TestCloseLocalBoardHTTPServerClosesActiveSSEImmediately(t *testing.T) {
	store, err := localboard.Open(context.Background(), localboard.Options{DBPath: filepath.Join(t.TempDir(), ".jjc", "local-board.sqlite")})
	if err != nil {
		t.Fatalf("open local board: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	srv := newLocalBoardHTTPServer("127.0.0.1:0", localboard.NewHandler(store, nil))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/events")
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	defer resp.Body.Close()

	done := make(chan struct{})
	go func() {
		closeLocalBoardHTTPServer(srv, sysevent.FromLogger(log.New(io.Discard, "", 0)))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("local board shutdown blocked on active SSE connection")
	}
}
