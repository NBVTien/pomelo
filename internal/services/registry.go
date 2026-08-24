package services

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pomelohq/pomelo/internal/paths"
)

type Registry struct {
	Projects map[string]string `json:"projects"`
}

func registryPath() string { return paths.StatePath("registry.json") }

func LoadRegistry() Registry {
	data, err := os.ReadFile(registryPath())
	if err != nil {
		return Registry{Projects: make(map[string]string)}
	}
	var reg Registry
	if json.Unmarshal(data, &reg) != nil {
		return Registry{Projects: make(map[string]string)}
	}
	if reg.Projects == nil {
		reg.Projects = make(map[string]string)
	}
	return reg
}

func saveRegistry(reg *Registry) {
	path := registryPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(reg, "", "  ")
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

func RegisterProject(session, projectDir string) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return
	}
	reg := LoadRegistry()
	if reg.Projects[session] == absDir {
		return
	}
	reg.Projects[session] = absDir
	saveRegistry(&reg)
}

func ProjectDir(session string) (string, bool) {
	reg := LoadRegistry()
	dir, ok := reg.Projects[session]
	return dir, ok
}

func ListProjects() map[string]string {
	return LoadRegistry().Projects
}

func UnregisterProject(session string) {
	reg := LoadRegistry()
	delete(reg.Projects, session)
	saveRegistry(&reg)
}
