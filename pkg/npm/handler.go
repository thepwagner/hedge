package npm

import (
	"net/http"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	log    logr.Logger
	tracer trace.Tracer
}

func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/npm/{dist}/{package}", h.GetPackageDetails)
	// r.HandleFunc("/debian/dists/{dist}/{comp}/binary-{arch}/Packages{compression:(?:|.xz|.gz)}", h.HandlePackages)
	// r.HandleFunc("/debian/dists/{dist}/pool/{path:.*}", h.HandlePool)
}

func (h *Handler) GetPackageDetails(w http.ResponseWriter, r *http.Request) {
	_, span := h.tracer.Start(r.Context(), "GetPackageDetails")
	defer span.End()

	vars := mux.Vars(r)
	dist := vars["dist"]
	packageName := vars["package"]
	h.log.Info("GetPackageDetails", "dist", dist, "package", packageName)

	w.WriteHeader(http.StatusOK)
}
