package cli

import (
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
)

func App(log logr.Logger) *cli.App {
	return &cli.App{
		Name:        "hedge",
		Description: "Package proxy",
		Commands: []*cli.Command{
			ServerCommand(log),
		},
	}
}
