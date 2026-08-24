package core

import (
	"net/http"

	"github.com/pomelohq/pomelo/internal/httpx"
)

func writeJSON(w http.ResponseWriter, v any) { httpx.Write(w, v) }

func readJSON[T any](r *http.Request) (T, error) { return httpx.Read[T](r) }

func httpErr(w http.ResponseWriter, code int, format string, a ...any) {
	httpx.Err(w, code, format, a...)
}
