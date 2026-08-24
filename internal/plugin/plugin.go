package plugin

import (
	"net/http"

	"github.com/spf13/cobra"
)

type Feature interface {
	Name() string
}

type HTTPProvider interface {
	Feature
	Routes(mux *http.ServeMux)
}

type CLIProvider interface {
	Feature
	Commands() []*cobra.Command
}
