package commands

import (
	"fmt"
	"time"

	"github.com/pomelohq/pomelo/internal/services"
)

func Ports(watch bool) {
	render := func() {
		leases := services.AllPortLeases()
		fmt.Printf("%s%-6s  %-9s  %-30s  %-16s  %s%s\n", Bold, "PORT", "STATE", "SERVICE", "SESSION", "AGE", NC)
		if len(leases) == 0 {
			fmt.Printf("  %sno ports leased%s\n", Dim, NC)
			return
		}
		for _, l := range leases {
			ws, svc := services.SplitLeaseKey(l.Key)
			svcCol := svc
			if ws != "" && ws != "shared" {
				svcCol = ws + "/" + svc
			}
			age := time.Since(l.Since).Round(time.Second)
			fmt.Printf("  %-6d  %s%-9s%s  %-30s  %-16s  %s\n",
				l.Port, portStateColor(l.State), l.State, NC, svcCol, l.Session, age)
		}
	}

	if !watch {
		render()
		return
	}
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("%spom ports%s — live (Ctrl+C to stop)\n\n", Bold, NC)
		render()
		time.Sleep(1500 * time.Millisecond)
	}
}

func portStateColor(s services.PortState) string {
	switch s {
	case services.PortRunning:
		return Green
	case services.PortStarting:
		return Yellow
	default:
		return Dim
	}
}
