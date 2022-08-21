package main

import (
	"os"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/thepwagner/hedge/pkg/cli"
)

func main() {
	zl := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.TimeFormat = time.RFC3339Nano
	})).With().Timestamp().Logger()
	log := zerologr.New(&zl)

	app := cli.App(log)
	if err := app.Run(os.Args); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
}
