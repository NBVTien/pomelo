package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) ensureSharedServices() {
	if s.cfg() == nil || s.WorkspaceRoot == "" || len(s.cfg().SharedServices) == 0 {
		return
	}
	composeFile := services.GenerateSharedCompose(s.WorkspaceRoot, s.cfg().Session, s.cfg().SharedServices)
	project := s.cfg().Session + "-shared"
	_ = exec.Command("docker", "compose", "-f", composeFile, "-p", project, "up", "-d").Run()
}

func (s *Server) handleSharedStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.SharedStatus())
}

func (s *Server) SharedStatus() map[string]any {
	running := map[string]bool{}
	if s.cfg() != nil && s.WorkspaceRoot != "" {
		composeFile := filepath.Join(s.WorkspaceRoot, "docker-compose.shared.yml")
		project := s.cfg().Session + "-shared"
		out, err := exec.Command("docker", "compose", "-f", composeFile, "-p", project,
			"ps", "--services", "--filter", "status=running").Output()
		if err == nil {
			for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if ln != "" {
					running[ln] = true
				}
			}
		}
	}
	urls := map[string]string{}
	type svcRow struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Image   string `json:"image"`
		Port    int    `json:"port"`
		Running bool   `json:"running"`
		URL     string `json:"url"`
	}
	list := []svcRow{}
	if cfg := s.cfg(); cfg != nil {
		order := cfg.SharedOrder
		if len(order) == 0 {
			for name := range cfg.SharedServices {
				order = append(order, name)
			}
		}
		for _, name := range order {
			def := cfg.SharedServices[name]
			if def == nil {
				continue
			}
			u := s.devProxySharedURL(cfg, name)
			if u != "" {
				urls[name] = u
			}
			typ := def.Type
			if typ == "" {
				typ = name
			}
			list = append(list, svcRow{
				Name: name, Type: typ, Image: def.Image,
				Port: services.SharedPort(name), Running: running[name], URL: u,
			})
		}
	}
	return map[string]any{"running": running, "urls": urls, "services": list}
}

func (s *Server) handleSharedAction(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, Action string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, http.StatusBadRequest, "bad json")
		return
	}
	writeJSON(w, s.SharedAction(req.Name, req.Action))
}

func (s *Server) SharedAction(name, action string) map[string]any {
	if s.cfg() == nil || s.WorkspaceRoot == "" {
		return map[string]any{"ok": false, "error": "no project"}
	}
	if _, ok := s.cfg().SharedServices[name]; !ok {
		return map[string]any{"ok": false, "error": "unknown shared service"}
	}
	composeFile := services.GenerateSharedCompose(s.WorkspaceRoot, s.cfg().Session, s.cfg().SharedServices)
	project := s.cfg().Session + "-shared"
	var args []string
	switch action {
	case "start":
		args = []string{"compose", "-f", composeFile, "-p", project, "up", "-d", name}
	case "restart":
		args = []string{"compose", "-f", composeFile, "-p", project, "up", "-d", "--force-recreate", name}
	case "stop":
		args = []string{"compose", "-f", composeFile, "-p", project, "stop", name}
	default:
		return map[string]any{"ok": false, "error": "action must be start|stop|restart"}
	}
	if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		return map[string]any{"ok": false, "error": strings.TrimSpace(string(out))}
	}
	return map[string]any{"ok": true}
}

func (s *Server) handleSharedStack(w http.ResponseWriter, r *http.Request) {
	var req struct{ Action string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, http.StatusBadRequest, "bad json")
		return
	}
	writeJSON(w, s.SharedStack(req.Action))
}

func (s *Server) SharedStack(action string) map[string]any {
	if s.cfg() == nil || s.WorkspaceRoot == "" || len(s.cfg().SharedServices) == 0 {
		return map[string]any{"ok": false, "error": "no shared services"}
	}
	composeFile := filepath.Join(s.WorkspaceRoot, "docker-compose.shared.yml")
	project := s.cfg().Session + "-shared"
	var args []string
	switch action {
	case "up":
		services.GenerateSharedCompose(s.WorkspaceRoot, s.cfg().Session, s.cfg().SharedServices)
		args = []string{"compose", "-f", composeFile, "-p", project, "up", "-d"}
	case "stop":
		args = []string{"compose", "-f", composeFile, "-p", project, "stop"}
	case "down":
		args = []string{"compose", "-f", composeFile, "-p", project, "down"}
	default:
		return map[string]any{"ok": false, "error": "action must be up|stop|down"}
	}
	if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		return map[string]any{"ok": false, "error": strings.TrimSpace(string(out))}
	}
	return map[string]any{"ok": true}
}

func (s *Server) handleSharedInspect(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.SharedInspect(r.URL.Query().Get("name")))
}

func (s *Server) SharedInspect(name string) map[string]any {
	if s.cfg() == nil || s.WorkspaceRoot == "" {
		return map[string]any{"error": "no project"}
	}
	def := s.cfg().SharedServices[name]
	if def == nil {
		return map[string]any{"error": "unknown shared service"}
	}
	composeFile := filepath.Join(s.WorkspaceRoot, "docker-compose.shared.yml")
	project := s.cfg().Session + "-shared"
	out, _ := exec.Command("docker", "compose", "-f", composeFile, "-p", project, "ps", "-q", name).Output()
	cid := strings.TrimSpace(string(out))

	resp := map[string]any{
		"name":  name,
		"image": def.Image,
		"port":  services.SharedPort(name),
		"url":   s.devProxySharedURL(s.cfg(), name),
	}
	if cid == "" {
		resp["running"] = false
		return resp
	}
	resp["running"] = true
	resp["id"] = cid
	if raw, err := exec.Command("docker", "inspect", cid).Output(); err == nil {
		var arr []struct {
			State  struct{ Status, StartedAt string }
			Config struct {
				Image  string
				Labels map[string]string
			}
			NetworkSettings struct {
				Networks map[string]struct{ IPAddress string }
				Ports    map[string][]struct{ HostIp, HostPort string }
			}
			Mounts []struct{ Source, Destination string }
		}
		if json.Unmarshal(raw, &arr) == nil && len(arr) == 1 {
			d := arr[0]
			resp["status"] = d.State.Status
			resp["started_at"] = d.State.StartedAt
			if d.Config.Image != "" {
				resp["image"] = d.Config.Image
			}
			for _, n := range d.NetworkSettings.Networks {
				if n.IPAddress != "" {
					resp["ip"] = n.IPAddress
					break
				}
			}
			ports := []map[string]string{}
			for cp, binds := range d.NetworkSettings.Ports {
				proto := "tcp"
				container := cp
				if i := strings.IndexByte(cp, '/'); i >= 0 {
					container, proto = cp[:i], cp[i+1:]
				}
				host := ""
				if len(binds) > 0 {
					host = binds[0].HostPort
				}
				ports = append(ports, map[string]string{"host": host, "container": container, "proto": proto})
			}
			sort.Slice(ports, func(i, j int) bool { return ports[i]["container"] < ports[j]["container"] })
			resp["ports"] = ports
			mounts := []map[string]string{}
			for _, m := range d.Mounts {
				mounts = append(mounts, map[string]string{"src": m.Source, "dst": m.Destination})
			}
			resp["mounts"] = mounts
			labels := []map[string]string{}
			keys := make([]string, 0, len(d.Config.Labels))
			for k := range d.Config.Labels {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				labels = append(labels, map[string]string{"key": k, "value": d.Config.Labels[k]})
			}
			resp["labels"] = labels
		}
	}
	return resp
}

func (s *Server) handleSharedStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.SharedStats(r.URL.Query().Get("name")))
}

func (s *Server) SharedStats(name string) map[string]any {
	if s.cfg() == nil || s.WorkspaceRoot == "" {
		return map[string]any{"running": false}
	}
	composeFile := filepath.Join(s.WorkspaceRoot, "docker-compose.shared.yml")
	project := s.cfg().Session + "-shared"
	if name == "" {
		return s.sharedStatsTotal(composeFile, project)
	}
	if s.cfg().SharedServices[name] == nil {
		return map[string]any{"running": false, "error": "unknown shared service"}
	}
	out, _ := exec.Command("docker", "compose", "-f", composeFile, "-p", project, "ps", "-q", name).Output()
	cid := strings.TrimSpace(string(out))
	resp := map[string]any{"running": cid != ""}
	if cid != "" {
		if st, err := exec.Command("docker", "stats", "--no-stream", "--format",
			"{{.CPUPerc}}|{{.MemUsage}}|{{.NetIO}}|{{.BlockIO}}", cid).Output(); err == nil {
			f := strings.SplitN(strings.TrimSpace(string(st)), "|", 4)
			if len(f) == 4 {
				resp["cpu"], resp["mem"], resp["net"], resp["disk"] = f[0], f[1], f[2], f[3]
			}
		}
	}
	return resp
}

func (s *Server) sharedStatsTotal(composeFile, project string) map[string]any {
	out, _ := exec.Command("docker", "compose", "-f", composeFile, "-p", project, "ps", "-q").Output()
	cids := strings.Fields(strings.TrimSpace(string(out)))
	resp := map[string]any{"running": len(cids), "cpu": "0%", "mem": "0B"}
	if len(cids) == 0 {
		return resp
	}
	args := append([]string{"stats", "--no-stream", "--format", "{{.CPUPerc}}|{{.MemUsage}}"}, cids...)
	st, err := exec.Command("docker", args...).Output()
	if err != nil {
		return resp
	}
	var cpu, memBytes float64
	for _, ln := range strings.Split(strings.TrimSpace(string(st)), "\n") {
		f := strings.SplitN(ln, "|", 2)
		if len(f) != 2 {
			continue
		}
		cpu += parsePercent(f[0])
		if used := strings.TrimSpace(strings.SplitN(f[1], "/", 2)[0]); used != "" {
			memBytes += parseByteSize(used)
		}
	}
	resp["cpu"] = fmt.Sprintf("%.1f%%", cpu)
	resp["mem"] = humanBytes(memBytes)
	return resp
}

func parsePercent(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%")), 64)
	return f
}

func parseByteSize(s string) float64 {
	units := []struct {
		suf string
		mul float64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1}}
	for _, u := range units {
		if strings.HasSuffix(s, u.suf) {
			n, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, u.suf)), 64)
			return n * u.mul
		}
	}
	return 0
}

func humanBytes(b float64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2fGiB", b/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0fMiB", b/(1<<20))
	default:
		return fmt.Sprintf("%.0fKiB", b/(1<<10))
	}
}

func (s *Server) handleSharedLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.SharedLogs(r.URL.Query().Get("name"), r.URL.Query().Get("lines")))
}

func (s *Server) SharedLogs(name, linesArg string) map[string]any {
	if s.cfg() == nil || s.WorkspaceRoot == "" {
		return map[string]any{"running": false, "error": "no project"}
	}
	if name == "" {
		return map[string]any{"running": false, "error": "name required"}
	}
	if _, ok := s.cfg().SharedServices[name]; !ok {
		return map[string]any{"running": false, "error": "unknown shared service"}
	}
	lines := 200
	if v, err := strconv.Atoi(linesArg); err == nil && v > 0 && v <= 2000 {
		lines = v
	}
	composeFile := filepath.Join(s.WorkspaceRoot, "docker-compose.shared.yml")
	project := s.cfg().Session + "-shared"
	out, err := exec.Command("docker", "compose", "-f", composeFile, "-p", project,
		"logs", "--no-color", "--tail", strconv.Itoa(lines), name).CombinedOutput()

	running := err == nil
	text := strings.TrimRight(string(out), "\n")
	var cleaned []string
	for _, ln := range strings.Split(text, "\n") {
		if i := strings.Index(ln, "| "); i >= 0 && i < 40 {
			ln = ln[i+2:]
		}
		cleaned = append(cleaned, ln)
	}
	if len(cleaned) == 1 && cleaned[0] == "" {
		cleaned = nil
	}
	return map[string]any{"running": running, "lines": cleaned}
}
