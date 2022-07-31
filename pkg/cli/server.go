package cli

import (
	"github.com/go-logr/logr"
	"github.com/thepwagner/hedge/pkg/server"
	"github.com/urfave/cli/v2"
)

const (
	flagConfigDirectory = "config-directory"
)

func ServerCommand(log logr.Logger) *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "Run the server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagConfigDirectory,
				Value: "./testconfig/",
			},
		},
		Action: func(c *cli.Context) error {
			cfgDir := c.String(flagConfigDirectory)
			cfg, err := server.LoadConfig(cfgDir)
			if err != nil {
				return err
			}
			log.V(1).Info("config loaded", "dir", cfgDir)

			return server.RunServer(log, *cfg)
		},
	}
}
