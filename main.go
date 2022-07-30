package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/thepwagner/hedge/debian"
)

type DefaultHandler struct {
	log logr.Logger
}

func (h *DefaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.log.V(1).Info("request received", "method", r.Method, "url", r.URL.String())
}

func run(log logr.Logger) error {
	r := mux.NewRouter()

	bullseyeKey, err := debian.ReadArmoredKeyRingFile("debian/testdata/bullseye_pubkey.txt")
	if err != nil {
		return err
	}
	remote := debian.NewRemoteLoader(log)
	remote.AddKeyring("bullseye", bullseyeKey)

	signingKey, err := debian.ReadArmoredKeyRingFile("testdata/priv.txt")
	if err != nil {
		return err
	}

	deb := debian.NewHandler(log, remote.Load, signingKey)
	deb.Register(r)

	h := &DefaultHandler{log: log}
	r.PathPrefix("/").Handler(h)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func main() {
	zl := zerolog.New(zerolog.NewConsoleWriter())
	log := zerologr.New(&zl)

	if err := run(log); err != nil {
		log.Error(err, "server error")
		panic(err)
	}
}
