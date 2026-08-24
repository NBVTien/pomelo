package core

import (
	"log"
	"time"

	"github.com/pomelohq/pomelo/internal/ptyhost"
	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) reapPortsLoop() {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	n := 0
	for range t.C {
		services.PortMgr().Reap()
		if n++; n%6 == 0 && s.WorkspaceRoot != "" {
			if reaped := ptyhost.ReapOrphanServices(s.WorkspaceRoot); len(reaped) > 0 {
				log.Printf("reaped %d orphaned service process(es): %v", len(reaped), reaped)
			}
		}
	}
}
