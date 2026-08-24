package commands

import (
	"fmt"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/services"
)

func Refresh(cfg *config.Config) error {
	n := services.StopSession(cfg.Session)
	if n == 0 {
		fmt.Printf("%s>>>%s no running services for '%s'\n", Blue, NC, cfg.Session)
		return nil
	}
	fmt.Printf("%s>>>%s stopped %d service(s) for %s%s%s — ports freed\n", Green, NC, n, Cyan, cfg.Session, NC)
	return nil
}
