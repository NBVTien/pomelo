package pombin

import (
	"os"
	"sync/atomic"
)

var override atomic.Pointer[string]

func Set(path string) {
	if path != "" {
		override.Store(&path)
	}
}

func Path() (string, error) {
	if p := override.Load(); p != nil {
		return *p, nil
	}
	return os.Executable()
}
