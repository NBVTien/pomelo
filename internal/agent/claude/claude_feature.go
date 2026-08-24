package claude

import (
	"net/http"
	"sync"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/plugin"
	"github.com/pomelohq/pomelo/internal/services"
)

type Feature struct {
	getCfg              func() *config.Config
	WorkspaceRoot       string
	mcpConfigJSONFn     func(branch string) string
	writeWorkspaceMapFn func(branch string, isMain bool)
	ticketContextFn     func(branch string) string

	hdMu     sync.Mutex
	headless map[string]*Driver

	agentMu      sync.Mutex
	agentSubs    map[chan []byte]struct{}
	agentStateMu sync.Mutex
	agentState   map[string]string
}

func New(getCfg func() *config.Config, workspaceRoot string, mcpConfigJSON func(string) string, writeWorkspaceMap func(string, bool), ticketContext func(string) string) *Feature {
	return &Feature{
		getCfg: getCfg, WorkspaceRoot: workspaceRoot,
		mcpConfigJSONFn: mcpConfigJSON, writeWorkspaceMapFn: writeWorkspaceMap, ticketContextFn: ticketContext,
		headless:   map[string]*Driver{},
		agentSubs:  map[chan []byte]struct{}{},
		agentState: map[string]string{},
	}
}

func (s *Feature) cfg() *config.Config { return s.getCfg() }
func (s *Feature) workspaceRoot(branch string, isMain bool) string {
	return services.WorkspaceRootDir(s.WorkspaceRoot, branch, isMain)
}

func (s *Feature) ResetForSwitch(newRoot string) {
	s.hdMu.Lock()
	drivers := s.headless
	s.headless = map[string]*Driver{}
	s.WorkspaceRoot = newRoot
	s.hdMu.Unlock()
	for _, d := range drivers {
		d.Stop()
		d.mu.Lock()
		cmd := d.cmd
		d.mu.Unlock()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}
func (s *Feature) getAgentState(key string) string {
	s.agentStateMu.Lock()
	defer s.agentStateMu.Unlock()
	return s.agentState[key]
}
func (s *Feature) mcpConfigJSON(branch string) string      { return s.mcpConfigJSONFn(branch) }
func (s *Feature) writeWorkspaceMap(branch string, m bool) { s.writeWorkspaceMapFn(branch, m) }
func (s *Feature) ticketContext(branch string) string      { return s.ticketContextFn(branch) }

func (*Feature) Name() string { return "claude" }

func (s *Feature) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/claude/session", s.handleClaudeSession)
	mux.HandleFunc("/api/claude/terminal", s.handleClaudeTerminal)
	mux.HandleFunc("/api/claude/hook", s.handleClaudeHook)
	mux.HandleFunc("/api/claude/commands", s.handleClaudeCommands)
	mux.HandleFunc("/api/claude/upload", s.handleClaudeUpload)
	mux.HandleFunc("/api/claude/image", s.handleClaudeImage)
	mux.HandleFunc("/api/agents/stream", s.handleAgentStream)
	mux.HandleFunc("/api/agents/state", s.handleAgentStates)
	mux.HandleFunc("/api/claude/transcript", s.handleTranscriptRange)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/agents/question", s.handleAgentQuestion)
}

var _ plugin.HTTPProvider = (*Feature)(nil)
