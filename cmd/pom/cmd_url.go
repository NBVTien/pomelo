package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/services"
)

var urlCmd = &cobra.Command{
	Use:   "url <session> <shared-service>",
	Short: "Print the connection URL for a shared service in a session",
	Long: `Resolve the connection URL for a shared service (postgres/redis/…) in a
registered session, so scripts don't have to hardcode ports.

  pom url acme postgres    # postgresql://user:pass@host:<random-port>
  pom url acme redis        # redis://host:<random-port>

For a database, append the workspace's db name, e.g.
  DB=$(pom url acme postgres)/acme_$(echo "$BRANCH" | tr '/-' '_')`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		u, err := services.SharedServiceURL(args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Println(u)
		return nil
	},
}
