package dbclient

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type Table struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type Column struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type Result struct {
	Columns      []string   `json:"columns"`
	Rows         [][]string `json:"rows"`
	Truncated    bool       `json:"truncated"`
	RowsAffected int64      `json:"rows_affected"`
}

type Driver interface {
	Ping(ctx context.Context) error
	Schemas(ctx context.Context) ([]string, error)
	Tables(ctx context.Context, schema string) ([]Table, error)
	Columns(ctx context.Context) ([]Column, error)
	Query(ctx context.Context, sql string, limit int) (*Result, error)
	ExportCSV(ctx context.Context, sql, path string) (rows int64, err error)
	Close() error
}

type Opener func(ctx context.Context, dsn string) (Driver, error)

var (
	mu      sync.RWMutex
	openers = map[string]Opener{}
)

func Register(engine string, o Opener) {
	mu.Lock()
	defer mu.Unlock()
	openers[engine] = o
}

func Engines() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(openers))
	for k := range openers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func Open(ctx context.Context, engine, dsn string) (Driver, error) {
	mu.RLock()
	o := openers[engine]
	mu.RUnlock()
	if o == nil {
		return nil, fmt.Errorf("unsupported db engine %q (have %v)", engine, Engines())
	}
	return o(ctx, dsn)
}
