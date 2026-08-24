package core

import (
	"net/http"
	"sync"
	"time"
)

type ProxyLogEntry struct {
	Time    string `json:"time"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Repo    string `json:"repo"`
	Svc     string `json:"svc"`
	Profile string `json:"profile"`
	Target  string `json:"target"`
	Status  int    `json:"status"`
	Ms      int64  `json:"ms"`
}

type proxyLog struct {
	mu  sync.Mutex
	buf []ProxyLogEntry
}

const proxyLogCap = 300

func (l *proxyLog) add(e ProxyLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, e)
	if len(l.buf) > proxyLogCap {
		l.buf = l.buf[len(l.buf)-proxyLogCap:]
	}
}

func (l *proxyLog) snapshot(limit int) []ProxyLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(l.buf)
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]ProxyLogEntry, 0, limit)
	for i := n - 1; i >= n-limit; i-- {
		out = append(out, l.buf[i])
	}
	return out
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (s *Server) handleDevProxyLog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.DevProxyLog(parseLimit(r, 200)))
}

func (s *Server) DevProxyLog(limit int) map[string]any {
	return map[string]any{"entries": s.proxyLog.snapshot(limit)}
}

func parseLimit(r *http.Request, def int) int {
	q := r.URL.Query().Get("limit")
	if q == "" {
		return def
	}
	n := 0
	for _, c := range q {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return def
	}
	return n
}

func nowStamp() string { return time.Now().Format("15:04:05") }
