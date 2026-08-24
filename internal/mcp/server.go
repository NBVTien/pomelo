package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"
)

type client struct {
	base    string
	handler http.Handler
	branch  string
	http    *http.Client
	pm      string
}

func newClient(base, branch string) *client {
	return &client{base: strings.TrimRight(base, "/"), branch: branch,
		http: &http.Client{Timeout: 6 * time.Minute}}
}

func newClientHandler(h http.Handler, branch string) *client {
	return &client{handler: h, branch: branch}
}

func (c *client) do(method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	var data []byte
	var status int
	if c.handler != nil {
		req := httptest.NewRequest(method, "http://mcp"+path, rdr)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		c.handler.ServeHTTP(rec, req)
		data, status = rec.Body.Bytes(), rec.Code
	} else {
		req, err := http.NewRequest(method, c.base+path, rdr)
		if err != nil {
			return err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("pom dashboard not reachable at %s — is `pom` running? (%w)", c.base, err)
		}
		defer resp.Body.Close()
		data, _ = io.ReadAll(resp.Body)
		status = resp.StatusCode
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s", strings.TrimSpace(string(data)))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

type wsSvc struct {
	Name       string `json:"name"`
	Running    bool   `json:"running"`
	Port       int    `json:"port,omitempty"`
	Mode       string `json:"mode,omitempty"`
	TmuxWindow string `json:"tmux_window,omitempty"`
	AgentName  string `json:"agent_name,omitempty"`
	AgentState string `json:"agent_state,omitempty"`
}
type wsShortcut struct {
	Cmd  string `json:"cmd"`
	Desc string `json:"desc"`
	Key  string `json:"key,omitempty"`
}
type wsRepo struct {
	Name      string       `json:"name"`
	Alias     string       `json:"alias"`
	Path      string       `json:"path"`
	Services  []wsSvc      `json:"services"`
	Shortcuts []wsShortcut `json:"shortcuts,omitempty"`
	Setup     []string     `json:"setup,omitempty"`
}
type wsEntry struct {
	Branch     string   `json:"branch"`
	IsMain     bool     `json:"is_main"`
	Path       string   `json:"path"`
	Repos      []wsRepo `json:"repos"`
	WsServices []wsSvc  `json:"ws_services"`
	Running    int      `json:"running"`
	Total      int      `json:"total"`
}

func (c *client) workspace() (*wsEntry, error) {
	var resp struct {
		Workspaces []wsEntry `json:"workspaces"`
		LocalPM    string    `json:"local_pm"`
	}
	if err := c.do(http.MethodGet, "/api/workspaces", nil, &resp); err != nil {
		return nil, err
	}
	c.pm = resp.LocalPM
	for i := range resp.Workspaces {
		if resp.Workspaces[i].Branch == c.branch {
			return &resp.Workspaces[i], nil
		}
	}
	return nil, fmt.Errorf("no workspace for branch %q — create it first", c.branch)
}

func (ws *wsEntry) resolveSvc(repo, service string) (refRepo string, svc *wsSvc) {
	if repo == "" || repo == "_ws" {
		for i := range ws.WsServices {
			if ws.WsServices[i].Name == service {
				return "_ws", &ws.WsServices[i]
			}
		}
		return "", nil
	}
	for i := range ws.Repos {
		r := &ws.Repos[i]
		if r.Name != repo && r.Alias != repo {
			continue
		}
		for j := range r.Services {
			if r.Services[j].Name == service {
				return r.Name, &r.Services[j]
			}
		}
	}
	return "", nil
}

func jsonText(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	return string(b), err
}

func str(args map[string]any, k string) string {
	if v, ok := args[k].(string); ok {
		return v
	}
	return ""
}

func ToolsHandler(h http.Handler, branch string) []Tool { return toolsFor(newClientHandler(h, branch)) }

func Tools(base, branch string) []Tool { return toolsFor(newClient(base, branch)) }

func toolsFor(c *client) []Tool {
	branch := c.branch

	svcArg := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"service": map[string]any{"type": "string", "description": "service name"},
			"repo":    map[string]any{"type": "string", "description": "repo name/alias; omit for workspace-level services (Claude Code, editor)"},
		},
		"required": []string{"service"},
	}

	action := func(name string) Tool {
		return Tool{
			Name:        "service_" + name,
			Description: name + " a service in this workspace and report status. Ports are pre-flighted, so a started service is guaranteed to bind the port pom reports.",
			Schema:      svcArg,
			Run: func(a map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				refRepo, svc := ws.resolveSvc(str(a, "repo"), str(a, "service"))
				if svc == nil {
					return "", fmt.Errorf("no service %q in this workspace", str(a, "service"))
				}
				body := map[string]any{"branch": branch, "is_main": ws.IsMain, "repo": refRepo, "svc": str(a, "service")}
				if err := c.do(http.MethodPost, "/api/service/"+name, body, nil); err != nil {
					return "", err
				}
				return fmt.Sprintf("%s: %s (port %d)", name, str(a, "service"), svc.Port), nil
			},
		}
	}

	return []Tool{
		{
			Name:        "workspace_info",
			ReadOnly:    true,
			Description: "Overview of THIS workspace: branch, repos, and their services (running state, ports, modes, agent state).",
			Run: func(map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				return jsonText(ws)
			},
		},
		{
			Name:        "services",
			ReadOnly:    true,
			Description: "List this workspace's services with running state and allocated port.",
			Run: func(map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				type row struct {
					Repo, Service string
					Running       bool
					Port          int
				}
				var rows []row
				for _, r := range ws.Repos {
					for _, s := range r.Services {
						rows = append(rows, row{r.Name, s.Name, s.Running, s.Port})
					}
				}
				return jsonText(rows)
			},
		},
		{
			Name:        "ports",
			ReadOnly:    true,
			Description: "Map of this workspace's services to their allocated localhost ports.",
			Run: func(map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				ports := map[string]int{}
				for _, r := range ws.Repos {
					for _, s := range r.Services {
						if s.Port > 0 {
							ports[r.Name+"/"+s.Name] = s.Port
						}
					}
				}
				return jsonText(ports)
			},
		},
		{
			Name:        "service_url",
			ReadOnly:    true,
			Description: "The base URL to reach a service (dev-proxy domain if configured, else http://localhost:<port>). Curl it via run_in_env, e.g. `curl -s $(url)/health`.",
			Schema:      svcArg,
			Run: func(a map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				refRepo, svc := ws.resolveSvc(str(a, "repo"), str(a, "service"))
				if svc == nil {
					return "", fmt.Errorf("no service %q in this workspace", str(a, "service"))
				}
				body := map[string]any{"branch": branch, "is_main": ws.IsMain, "repo": refRepo, "svc": str(a, "service")}
				var out struct {
					URL string `json:"url"`
				}
				if err := c.do(http.MethodPost, "/api/service/url", body, &out); err != nil {
					return "", err
				}
				if out.URL == "" {
					return "", fmt.Errorf("no URL for %q (not running and no remote env?)", str(a, "service"))
				}
				return out.URL, nil
			},
		},
		{
			Name:        "databases",
			ReadOnly:    true,
			Description: "This branch's databases with ready-to-use Postgres connection strings. Use these to run migrations/queries against the per-branch DB.",
			Run: func(map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodGet, "/api/databases?branch="+url.QueryEscape(branch), nil, &resp); err != nil {
					return "", err
				}
				return jsonText(resp["databases"])
			},
		},
		{
			Name:        "db_list",
			ReadOnly:    true,
			Description: "List the databases you can browse in THIS branch, each with `name` (use it as `db` in db_tables/db_columns/db_query), `engine` (postgres|redis), `repo`, and `label`. Includes shared Redis keyspaces. Prefer this + db_query to inspect data over spinning up psql via run_in_env.",
			Run: func(map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodGet, "/api/db/list?branch="+url.QueryEscape(branch), nil, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("%v", resp["error"])
				}
				return jsonText(resp["databases"])
			},
		},
		{
			Name:        "db_tables",
			ReadOnly:    true,
			Description: "List a database's tables/views (postgres) or keyspaces (redis). `db` is a `name` from db_list.",
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"db": map[string]any{"type": "string", "description": "database name from db_list"}},
				"required":   []string{"db"},
			},
			Run: func(a map[string]any) (string, error) {
				var resp map[string]any
				q := "/api/db/tables?branch=" + url.QueryEscape(branch) + "&db=" + url.QueryEscape(str(a, "db"))
				if err := c.do(http.MethodGet, q, nil, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("%v", resp["error"])
				}
				return jsonText(resp["tables"])
			},
		},
		{
			Name:        "db_columns",
			ReadOnly:    true,
			Description: "List every column (schema/table/name/type) in a database — use to learn the schema before writing a query. `db` is a `name` from db_list.",
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"db": map[string]any{"type": "string", "description": "database name from db_list"}},
				"required":   []string{"db"},
			},
			Run: func(a map[string]any) (string, error) {
				var resp map[string]any
				q := "/api/db/columns?branch=" + url.QueryEscape(branch) + "&db=" + url.QueryEscape(str(a, "db"))
				if err := c.do(http.MethodGet, q, nil, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("%v", resp["error"])
				}
				return jsonText(resp["columns"])
			},
		},
		{
			Name:           "db_query",
			MaxResultChars: 12000,
			Description:    "Run SQL against a branch database (postgres) or a command against Redis, returning columns + rows. `db` is a `name` from db_list. Reads are safe; writes hit the REAL per-branch DB — verify with a SELECT first. Results are capped by `limit` (default 200).",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"db":    map[string]any{"type": "string", "description": "database name from db_list"},
					"sql":   map[string]any{"type": "string", "description": "SQL (postgres) or a Redis command (e.g. `GET key`)"},
					"limit": map[string]any{"type": "integer", "description": "max rows (default 200)"},
				},
				"required": []string{"db", "sql"},
			},
			Run: func(a map[string]any) (string, error) {
				limit := 200
				if n, ok := a["limit"].(float64); ok && n > 0 {
					limit = int(n)
				}
				body := map[string]any{"branch": branch, "db": str(a, "db"), "sql": str(a, "sql"), "limit": limit}
				var resp map[string]any
				if err := c.do(http.MethodPost, "/api/db/query", body, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("%v", resp["error"])
				}
				out := map[string]any{"columns": resp["columns"], "rows": resp["rows"]}
				if t, _ := resp["truncated"].(bool); t {
					out["truncated"] = true
				}
				if n, _ := resp["rows_affected"].(float64); n > 0 {
					out["rows_affected"] = n
				}
				return jsonText(out)
			},
		},
		action("start"),
		action("stop"),
		action("restart"),
		{
			Name:        "commands",
			ReadOnly:    true,
			Description: "The project's PRE-WRITTEN commands per repo: one-time `setup` steps and `shortcuts` (each with `key`→`desc`→`cmd`), already preset-resolved. ALWAYS check here before hand-writing a shell command — these carry the project's canonical install / generate / migrate / lint / test / build invocations (e.g. `npx prisma generate`, `bundle exec rake db:migrate`). Run one with `run_shortcut` (by `key` when present, else `desc`) or, for a raw setup step, `run_in_env`.",
			Run: func(map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				type repoCmds struct {
					Repo      string       `json:"repo"`
					Alias     string       `json:"alias,omitempty"`
					Setup     []string     `json:"setup,omitempty"`
					Shortcuts []wsShortcut `json:"shortcuts,omitempty"`
				}
				var repos []repoCmds
				for _, r := range ws.Repos {
					if len(r.Setup) == 0 && len(r.Shortcuts) == 0 {
						continue
					}
					repos = append(repos, repoCmds{Repo: r.Name, Alias: r.Alias, Setup: r.Setup, Shortcuts: r.Shortcuts})
				}
				out := map[string]any{"repos": repos}
				if c.pm != "" {
					out["package_manager"] = c.pm
					out["package_manager_note"] = "This project uses " + c.pm +
						". pom auto-rewrites `npm install`/`yarn`/`npm ci` to `" + c.pm +
						" install --shamefully-hoist` in run_in_env and run_shortcut, but you should also use `" +
						c.pm + "` (not npm/yarn) for run/add/exec and any other package commands."
				}
				if len(repos) == 0 && c.pm == "" {
					return "No pre-written setup/shortcuts in this project's config.", nil
				}
				return jsonText(out)
			},
		},
		{
			Name:           "run_shortcut",
			MaxResultChars: 8000,
			Description:    "Run one of a repo's PRE-WRITTEN shortcuts, in the repo's worktree with the workspace's resolved env. Address it by `key` (the canonical op — install/generate/migrate/test/lint/build/format; see each shortcut's `key` in `commands`) OR by `desc`. PREFER `key` when it exists — it's stable across projects. PREFER this over `run_in_env` whenever a shortcut exists — it uses the project's exact, tested command. Synchronous.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repo": map[string]any{"type": "string", "description": "repo name/alias"},
					"key":  map[string]any{"type": "string", "description": "canonical op: install/generate/migrate/test/lint/build/format"},
					"desc": map[string]any{"type": "string", "description": "the shortcut's description, or a unique substring of it (use when there's no key)"},
				},
				"required": []string{"repo"},
			},
			Run: func(a map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				repo, key, want := str(a, "repo"), str(a, "key"), str(a, "desc")
				if key == "" && want == "" {
					return "", fmt.Errorf("provide `key` (e.g. migrate) or `desc` — call `commands` to list them")
				}
				var r *wsRepo
				for i := range ws.Repos {
					if ws.Repos[i].Name == repo || ws.Repos[i].Alias == repo {
						r = &ws.Repos[i]
						break
					}
				}
				if r == nil {
					return "", fmt.Errorf("no repo %q in this workspace", repo)
				}
				cmd := ""
				if key != "" {
					for _, sc := range r.Shortcuts {
						if strings.EqualFold(sc.Key, key) {
							cmd = sc.Cmd
							break
						}
					}
					if cmd == "" {
						return "", fmt.Errorf("no command %q in %s — call `commands` to list keys", key, repo)
					}
				} else {
					for _, sc := range r.Shortcuts {
						if strings.EqualFold(sc.Desc, want) {
							cmd = sc.Cmd
							break
						}
					}
					if cmd == "" {
						for _, sc := range r.Shortcuts {
							if strings.Contains(strings.ToLower(sc.Desc), strings.ToLower(strings.TrimSpace(want))) {
								cmd = sc.Cmd
								break
							}
						}
					}
					if cmd == "" {
						return "", fmt.Errorf("no shortcut matching %q in %s — call `commands` to list them", want, repo)
					}
				}
				body := map[string]any{"branch": branch, "is_main": ws.IsMain, "repo": r.Name, "cmd": cmd}
				var resp struct {
					Exit   int    `json:"exit"`
					Output string `json:"output"`
				}
				if err := c.do(http.MethodPost, "/api/run-in-env", body, &resp); err != nil {
					return "", err
				}
				return fmt.Sprintf("$ %s\nexit %d\n%s", cmd, resp.Exit, resp.Output), nil
			},
		},
		{
			Name:           "service_logs",
			MaxResultChars: 8000,
			ReadOnly:       true,
			Description:    "Recent terminal output of a service (to check for errors like a port-in-use).",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"service": map[string]any{"type": "string"},
					"repo":    map[string]any{"type": "string"},
					"lines":   map[string]any{"type": "integer", "description": "how many trailing lines (default 200)"},
				},
				"required": []string{"service"},
			},
			Run: func(a map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				_, svc := ws.resolveSvc(str(a, "repo"), str(a, "service"))
				if svc == nil || svc.TmuxWindow == "" {
					return "", fmt.Errorf("service %q not found or not started", str(a, "service"))
				}
				lines := 200
				if n, ok := a["lines"].(float64); ok && n > 0 {
					lines = int(n)
				}
				q := "/api/service/peek?window=" + url.QueryEscape(svc.TmuxWindow) + fmt.Sprintf("&lines=%d", lines)
				var resp struct {
					Running bool     `json:"running"`
					Lines   []string `json:"lines"`
				}
				if err := c.do(http.MethodGet, q, nil, &resp); err != nil {
					return "", err
				}
				return strings.Join(resp.Lines, "\n"), nil
			},
		},
		{
			Name:           "run_in_env",
			MaxResultChars: 8000,
			Description:    "Run a shell command in a repo's worktree with the workspace's resolved env (correct DATABASE_URL/ports). Use to run migrations, tests, seeds and verify against the REAL running stack. Synchronous, 5-min timeout. NOTE: if the project already defines a `setup` step or `shortcut` for this task (call `commands` to check), prefer `run_shortcut` so you use the project's canonical invocation instead of guessing one.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cmd":  map[string]any{"type": "string", "description": "shell command"},
					"repo": map[string]any{"type": "string", "description": "repo name/alias to run in"},
				},
				"required": []string{"cmd", "repo"},
			},
			Run: func(a map[string]any) (string, error) {
				ws, err := c.workspace()
				if err != nil {
					return "", err
				}
				body := map[string]any{"branch": branch, "is_main": ws.IsMain, "repo": str(a, "repo"), "cmd": str(a, "cmd")}
				var resp struct {
					Exit   int    `json:"exit"`
					Output string `json:"output"`
				}
				if err := c.do(http.MethodPost, "/api/run-in-env", body, &resp); err != nil {
					return "", err
				}
				return fmt.Sprintf("exit %d\n%s", resp.Exit, resp.Output), nil
			},
		},
		{
			Name:        "resolve_port_conflict",
			Description: "Move this workspace to a fresh, fully-free port region and regenerate its env — the self-heal when a service can't bind because something grabbed pom's port. Restart affected services afterward.",
			Run: func(map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodPost, "/api/ports/relocate", map[string]any{"branch": branch}, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("%v", resp["error"])
				}
				return "Relocated to a clean port region and regenerated env. Restart your services to pick up the new ports; call `ports` to see them.", nil
			},
		},
		{
			Name:        "config_get",
			ReadOnly:    true,
			Description: "Read this project's pom.yml (services, repos, shared services, env profiles, databases).",
			Run: func(map[string]any) (string, error) {
				var resp struct {
					YAML string `json:"yaml"`
				}
				if err := c.do(http.MethodGet, "/api/config", nil, &resp); err != nil {
					return "", err
				}
				return resp.YAML, nil
			},
		},
		{
			Name:        "secrets_list",
			ReadOnly:    true,
			Description: "List the NAMES of secrets in the app-local secret store (values are never returned). Onboarding imports a project's gitignored .env values here — wire each into the config env as {{secret.NAME}} (a real secret) or map infra to {{shared.*}} instead.",
			Run: func(map[string]any) (string, error) {
				var resp struct {
					Names []string `json:"names"`
				}
				if err := c.do(http.MethodGet, "/api/secrets", nil, &resp); err != nil {
					return "", err
				}
				return jsonText(resp.Names)
			},
		},
		{
			Name:        "config_doctor",
			ReadOnly:    true,
			Description: "Diagnose whether this project is runnable: returns structured findings (invalid config, missing docker/tools, missing repos, unset {{secret}}). Use this to drive a fix loop — after each config edit, call config_doctor again until it reports no errors.",
			Run: func(map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodGet, "/api/config/doctor", nil, &resp); err != nil {
					return "", err
				}
				return jsonText(resp)
			},
		},
		{
			Name:        "config_validate",
			ReadOnly:    true,
			Description: "Dry-run validate a proposed pom.yml (schema + reference checks) WITHOUT writing. Always validate before config_set.",
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"yaml": map[string]any{"type": "string"}},
				"required":   []string{"yaml"},
			},
			Run: func(a map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodPost, "/api/config/write", map[string]any{"yaml": str(a, "yaml"), "dry": true}, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); ok {
					return "valid", nil
				}
				return "", fmt.Errorf("invalid: %v", resp["error"])
			},
		},
		{
			Name:        "config_set",
			Description: "Validate and write a new pom.yml, then reload — adds/edits services, repos, shared services, databases, env. Rejected if invalid (nothing is written). Newly added services get ports allocated automatically.",
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"yaml": map[string]any{"type": "string"}},
				"required":   []string{"yaml"},
			},
			Run: func(a map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodPost, "/api/config/write", map[string]any{"yaml": str(a, "yaml")}, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("rejected: %v", resp["error"])
				}
				_ = c.do(http.MethodPost, "/api/config/reload", map[string]any{}, nil)
				return "Config written, validated, and reloaded.", nil
			},
		},
		{
			Name:        "config_normalize",
			Description: "Deterministically clean the config: strip REMOVED schema keys (schema_version/plugins/combinations/proxy/webhook/exposes), migrate legacy colon tokens to dot form, and tidy into pom.d fragments. Run this as the FINAL step of Adapt/onboarding — it does the mechanical cleanup so you don't have to.",
			Run: func(map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodPost, "/api/config/normalize", map[string]any{}, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("normalize failed: %v", resp["error"])
				}
				return jsonText(resp["removed"])
			},
		},
		{
			Name:        "config_split",
			Description: "Organize an inline pom.yml into pom.d/** fragments: one file per repo (pom.d/repos/NN-<name>.yml) plus environments/presets/shared_services each in their own file. Idempotent (re-running cleans stale repo fragments). Run this LAST, after the config is authored and validates clean, to keep the config tidy.",
			Run: func(map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodPost, "/api/config/split", map[string]any{}, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("split failed: %v", resp["error"])
				}
				return jsonText(resp["fragments"])
			},
		},
		{
			Name:        "config_files",
			ReadOnly:    true,
			Description: "List the config files: the root pom.yml plus every pom.d/**.yml fragment (when the config is split). Use this first when the config is split — edit the right fragment with config_file_get/config_file_set instead of config_set (which is rejected for split configs).",
			Run: func(map[string]any) (string, error) {
				var resp struct {
					Files []map[string]any `json:"files"`
				}
				if err := c.do(http.MethodGet, "/api/config/files", nil, &resp); err != nil {
					return "", err
				}
				return jsonText(resp.Files)
			},
		},
		{
			Name:        "config_file_get",
			ReadOnly:    true,
			Description: "Read one config file by its absolute path (from config_files) — the root pom.yml or a single pom.d fragment.",
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"path": map[string]any{"type": "string"}},
				"required":   []string{"path"},
			},
			Run: func(a map[string]any) (string, error) {
				var resp struct {
					YAML string `json:"yaml"`
				}
				if err := c.do(http.MethodGet, "/api/config/file?path="+url.QueryEscape(str(a, "path")), nil, &resp); err != nil {
					return "", err
				}
				return resp.YAML, nil
			},
		},
		{
			Name:        "config_file_set",
			Description: "Write one config file (root pom.yml or a pom.d fragment) by absolute path, then reload. The edit is validated against the FULL merged config before it lands — rejected (nothing written) if it breaks the whole. Prefer this over config_set when the config is split across pom.d.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
					"yaml": map[string]any{"type": "string"},
				},
				"required": []string{"path", "yaml"},
			},
			Run: func(a map[string]any) (string, error) {
				var resp map[string]any
				if err := c.do(http.MethodPost, "/api/config/file", map[string]any{"path": str(a, "path"), "yaml": str(a, "yaml")}, &resp); err != nil {
					return "", err
				}
				if ok, _ := resp["ok"].(bool); !ok {
					return "", fmt.Errorf("rejected: %v", resp["error"])
				}
				_ = c.do(http.MethodPost, "/api/config/reload", map[string]any{}, nil)
				return "File written, validated against the merged config, and reloaded.", nil
			},
		},
	}
}
