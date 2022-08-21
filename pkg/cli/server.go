package cli

import (
	"github.com/thepwagner/hedge/pkg/server"
	"github.com/urfave/cli/v2"
)

func ServerCommand() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "Run the server",
		Action: func(c *cli.Context) error {
			cfgDir := c.String(flagConfigDirectory)

			cfg, err := server.LoadConfig(cfgDir)
			if err != nil {
				return err
			}
			return server.RunServer(c.Context, *cfg)
		},
	}
}
