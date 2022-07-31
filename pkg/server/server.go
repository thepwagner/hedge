package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/debian"
	"github.com/thepwagner/hedge/pkg/observability"
)

func RunServer(log logr.Logger, cfg Config) error {
	r := mux.NewRouter()

	if len(cfg.Debian) > 0 {
		log.V(1).Info("enabled debian support", "debian_repos", len(cfg.Debian))
		h, err := debian.NewHandler(log, cfg.Debian...)
		if err != nil {
			return err
		}
		h.Register(r)
	}

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: observability.NewLoggingHandler(log, r),
	}
	log.Info("starting server", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
