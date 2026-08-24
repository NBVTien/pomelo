package core

import (
	"time"

	"github.com/pomelohq/pomelo/internal/commands"
)

func (s *Server) startResourceMonitor() {
	sampler := commands.NewResourceSampler()
	_ = sampler.Sample()
	go func() {
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		for range t.C {
			st := sampler.Sample()
			s.resMu.Lock()
			s.resStat = st
			s.resMu.Unlock()
		}
	}()
}

func (s *Server) resources() commands.ResourceStat {
	s.resMu.Lock()
	defer s.resMu.Unlock()
	return s.resStat
}
