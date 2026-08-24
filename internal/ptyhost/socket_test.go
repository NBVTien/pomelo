//go:build !windows

package ptyhost

import (
	"bytes"
	"io"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func serveCat(t *testing.T) (*Session, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s, err := Serve(ln, StartOpts{Argv: []string{"cat"}})
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	return s, path
}

func dial(t *testing.T, path string) net.Conn {
	t.Helper()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

func readUntil(t *testing.T, conn net.Conn, want string, d time.Duration) []byte {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(d))
	buf := []byte{}
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if bytes.Contains(buf, []byte(want)) {
			return buf
		}
		if err != nil {
			t.Fatalf("read (want %q): %v; got %q", want, err, buf)
		}
	}
}

func TestServePersistsAcrossReattach(t *testing.T) {
	s, path := serveCat(t)
	defer s.Close()

	a := dial(t, path)
	if err := WriteInput(a, []byte("hello-A\n")); err != nil {
		t.Fatalf("write A: %v", err)
	}
	readUntil(t, a, "hello-A", 5*time.Second)
	_ = a.Close()

	select {
	case <-s.Done():
		t.Fatal("session died when the client detached")
	default:
	}

	b := dial(t, path)
	defer b.Close()
	snap := readUntil(t, b, "hello-A", 5*time.Second)
	if !bytes.Contains(snap, []byte("hello-A")) {
		t.Fatalf("reattached client missed scrollback; got %q", snap)
	}
	if err := WriteInput(b, []byte("hello-B\n")); err != nil {
		t.Fatalf("write B: %v", err)
	}
	readUntil(t, b, "hello-B", 5*time.Second)
}

func TestServeLongName(t *testing.T) {
	name := "sh-ws:proj-1043-most-common-prospect-questions-widget-on-analytics-tab-plus-some-more-length"
	s, ln, err := ListenAndServe(name, StartOpts{Argv: []string{"cat"}})
	if err != nil {
		t.Fatalf("ListenAndServe long name: %v", err)
	}
	defer ln.Close()
	defer s.Close()

	c, err := net.Dial("unix", SocketPath(name))
	if err != nil {
		t.Fatalf("dial long-name socket: %v", err)
	}
	defer c.Close()
	if err := WriteInput(c, []byte("longname-ok\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	readUntil(t, c, "longname-ok", 5*time.Second)
}

func TestAttachDetachByte(t *testing.T) {
	s, path := serveCat(t)
	defer s.Close()

	conn := dial(t, path)
	defer conn.Close()

	in := bytes.NewReader([]byte("typed-xyz\n\x1c"))
	out := &syncBuf{}
	resize := make(chan [2]int)
	close(resize)

	errc := make(chan error, 1)
	go func() { errc <- Attach(conn, in, out, resize, 0x1c) }()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("attach returned error on detach: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("attach did not detach on the detach byte")
	}
	if !waitBuf(out, "typed-xyz", 3*time.Second) {
		t.Fatalf("input before detach was not forwarded; got %q", out.String())
	}
	select {
	case <-s.Done():
		t.Fatal("session died on client detach")
	default:
	}
}

func TestServeFanOutClients(t *testing.T) {
	s, path := serveCat(t)
	defer s.Close()

	a, b := dial(t, path), dial(t, path)
	defer a.Close()
	defer b.Close()

	if err := WriteInput(a, []byte("broadcast-1\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	readUntil(t, a, "broadcast-1", 5*time.Second)
	readUntil(t, b, "broadcast-1", 5*time.Second)
}

type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}
func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func waitBuf(s *syncBuf, want string, d time.Duration) bool {
	deadline := time.After(d)
	for {
		if bytes.Contains([]byte(s.String()), []byte(want)) {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-time.After(20 * time.Millisecond):
		}
	}
}

var _ io.Writer = (*syncBuf)(nil)
