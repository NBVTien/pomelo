package commands

import (
	"fmt"

	"github.com/pomelohq/pomelo/internal/services"
)

func Setup() error {
	fmt.Printf("%s[1/2] Port allocation%s\n", Bold, NC)
	fmt.Printf("%s>>>%s services bind %s; each gets a random free port (10000-65535), reserved while it runs and freed when it stops\n",
		Green, NC, services.BindIP())

	fmt.Printf("\n%s[2/2] Global gitignore%s\n", Bold, NC)
	services.EnsureGlobalGitignore()
	fmt.Printf("%s>>>%s global gitignore configured\n", Green, NC)

	fmt.Printf("\n%sSetup complete!%s\n", Green, NC)
	return nil
}
