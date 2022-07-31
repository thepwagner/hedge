package main

import (
	"os"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/thepwagner/hedge/pkg/cli"
)

func main() {
	zl := zerolog.New(zerolog.NewConsoleWriter())
	log := zerologr.New(&zl)

	app := cli.App(log)
	if err := app.Run(os.Args); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
}
