package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pomelohq/pomelo/internal/paths"
	"github.com/pomelohq/pomelo/internal/provider/dbclient"
	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) DBList(branch string) map[string]any {
	if s.cfg() == nil {
		return map[string]any{"ok": false, "error": "no project config"}
	}
	cfg := s.cfg()
	dbs := []map[string]any{}
	for key, dir := range cfg.Repos {
		repo := dir.Alias
		if repo == "" {
			repo = key
		}
		for alias, tpl := range dir.Databases {
			dbs = append(dbs, map[string]any{
				"name":   cfg.Session + "_" + services.ResolveBranchTokens(tpl, branch),
				"engine": "postgres", "repo": repo, "label": alias,
			})
		}
	}
	for _, name := range s.redisSharedNames() {
		dbs = append(dbs, map[string]any{"name": name, "engine": "redis", "repo": "shared", "label": name})
	}
	return map[string]any{"ok": true, "error": "", "databases": dbs}
}

func (s *Server) redisSharedNames() []string {
	var out []string
	for name, def := range s.cfg().SharedServices {
		if def.Type == "redis" || (def.Type == "" && name == "redis") {
			out = append(out, name)
		}
	}
	return out
}

func (s *Server) isRedis(db string) bool {
	for _, n := range s.redisSharedNames() {
		if n == db {
			return true
		}
	}
	return false
}

func (s *Server) dbDSN(branch, db string) (engine, dsn string) {
	if s.isRedis(db) {
		host := s.cfg().SharedHost(db)
		if host == "" {
			host = "localhost"
		}
		port := services.SharedPort(db)
		if port == 0 {
			port = 6379
		}
		slot := services.ResolveTokens("{{shared."+db+".slot}}",
			services.ResolveCtx{Cfg: s.cfg(), Branch: branch, WsKey: services.PortWsKey(branch)})
		if slot == "" {
			slot = "0"
		}
		return "redis", fmt.Sprintf("redis://%s:%d/%s", host, port, slot)
	}
	host, port, user, pw := s.pgConn()
	return "postgres", urlForDB(user, pw, host, int(port), db)
}

func (s *Server) DBTables(branch, db string) map[string]any {
	engine, dsn := s.dbDSN(branch, db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	d, err := dbclient.Open(ctx, engine, dsn)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	defer d.Close()
	tables, err := d.Tables(ctx, "")
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	if tables == nil {
		tables = []dbclient.Table{}
	}
	return map[string]any{"ok": true, "tables": tables}
}

func (s *Server) DBColumns(branch, db string) map[string]any {
	engine, dsn := s.dbDSN(branch, db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	d, err := dbclient.Open(ctx, engine, dsn)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	defer d.Close()
	cols, err := d.Columns(ctx)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	if cols == nil {
		cols = []dbclient.Column{}
	}
	return map[string]any{"ok": true, "columns": cols}
}

func (s *Server) DBQuery(branch, db, sql string, limit int) map[string]any {
	engine, dsn := s.dbDSN(branch, db)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d, err := dbclient.Open(ctx, engine, dsn)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	defer d.Close()
	res, err := d.Query(ctx, sql, limit)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true, "columns": res.Columns, "rows": res.Rows, "truncated": res.Truncated, "rows_affected": res.RowsAffected}
}

func (s *Server) DBExportCSV(branch, db, sql, path string) map[string]any {
	engine, dsn := s.dbDSN(branch, db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	d, err := dbclient.Open(ctx, engine, dsn)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	defer d.Close()
	n, err := d.ExportCSV(ctx, sql, path)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "rows": n}
	}
	return map[string]any{"ok": true, "rows": n, "path": path}
}

func (s *Server) dbConsolesPath() string {
	sess := "default"
	if s.cfg() != nil && s.cfg().Session != "" {
		sess = s.cfg().Session
	}
	return paths.StatePath(filepath.Join("db-consoles", sess+".json"))
}

func (s *Server) DBConsolesLoad() map[string]any {
	data, err := os.ReadFile(s.dbConsolesPath())
	if err != nil || len(data) == 0 {
		return map[string]any{"ok": true, "consoles": []any{}}
	}
	var consoles []any
	if err := json.Unmarshal(data, &consoles); err != nil {
		return map[string]any{"ok": true, "consoles": []any{}}
	}
	return map[string]any{"ok": true, "consoles": consoles}
}

func (s *Server) DBConsolesSave(data string) map[string]any {
	p := s.dbConsolesPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true}
}
