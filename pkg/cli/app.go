package cli

import (
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
)

const (
	flagConfigDirectory = "config-directory"
)

func App(log logr.Logger) *cli.App {
	return &cli.App{
		Name:        "hedge",
		Description: "Package proxy",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagConfigDirectory,
				Value: "./pkg/server/testdata/config/",
			},
		},
		Commands: []*cli.Command{
			ServerCommand(),
		},
	}
}
