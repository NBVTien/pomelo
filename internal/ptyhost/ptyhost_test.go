//go:build !windows

package ptyhost

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSessionRunsToCompletion(t *testing.T) {
	s, err := Start(StartOpts{Argv: []string{"sh", "-c", "printf hello-pty"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	snap, out, cancel := s.Subscribe()
	defer cancel()

	buf := append([]byte{}, snap...)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b, ok := <-out:
			if !ok {
				assertContains(t, buf, "hello-pty")
				return
			}
			buf = append(buf, b...)
			if bytes.Contains(buf, []byte("hello-pty")) {
				return
			}
		case <-deadline:
			t.Fatalf("timeout; got %q", buf)
		}
	}
}

func TestSessionInteractiveEcho(t *testing.T) {
	s, err := Start(StartOpts{Argv: []string{"cat"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	_, out, cancel := s.Subscribe()
	defer cancel()

	if _, err := s.Write([]byte("ping-123\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := []byte{}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b, ok := <-out:
			if !ok {
				t.Fatalf("closed before echo; got %q", buf)
			}
			buf = append(buf, b...)
			if bytes.Contains(buf, []byte("ping-123")) {
				return
			}
		case <-deadline:
			t.Fatalf("no echo; got %q", buf)
		}
	}
}

func TestSessionScrollbackSnapshot(t *testing.T) {
	s, err := Start(StartOpts{Argv: []string{"sh", "-c", "printf marker-42; sleep 2"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	time.Sleep(400 * time.Millisecond)
	snap, _, cancel := s.Subscribe()
	defer cancel()
	if !strings.Contains(string(snap), "marker-42") {
		t.Fatalf("scrollback missing prior output; got %q", snap)
	}
}

func TestSessionFanOut(t *testing.T) {
	s, err := Start(StartOpts{Argv: []string{"cat"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	_, a, ca := s.Subscribe()
	_, b, cb := s.Subscribe()
	defer ca()
	defer cb()

	s.Write([]byte("fan-out\n"))
	for _, out := range []<-chan []byte{a, b} {
		if !waitFor(out, "fan-out", 5*time.Second) {
			t.Fatal("a subscriber missed the output")
		}
	}
}

func TestSessionSmallestClientWins(t *testing.T) {
	s, err := Start(StartOpts{Argv: []string{"cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()

	big := s.AddClient()
	small := s.AddClient()
	s.ClientSize(big, 200, 60)
	s.ClientSize(small, 100, 40)

	s.szMu.Lock()
	c, r := s.szCols, s.szRows
	s.szMu.Unlock()
	if c != 100 || r != 40 {
		t.Fatalf("want min 100x40, got %dx%d", c, r)
	}

	s.RemoveClient(small)
	s.szMu.Lock()
	c, r = s.szCols, s.szRows
	s.szMu.Unlock()
	if c != 200 || r != 60 {
		t.Fatalf("after small detaches want 200x60, got %dx%d", c, r)
	}
}

func waitFor(out <-chan []byte, want string, d time.Duration) bool {
	buf := []byte{}
	deadline := time.After(d)
	for {
		select {
		case b, ok := <-out:
			if !ok {
				return false
			}
			buf = append(buf, b...)
			if bytes.Contains(buf, []byte(want)) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func assertContains(t *testing.T, buf []byte, want string) {
	t.Helper()
	if !bytes.Contains(buf, []byte(want)) {
		t.Fatalf("want %q in output; got %q", want, buf)
	}
}
