package api

import (
	"net/http"

	"github.com/google/uuid"
)

const (
	AccountName = "devstoreaccount1"
	AccountKey  = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	APIVersion  = "2024-08-04"
)

// AuthMiddleware adds standard Azure response headers and performs permissive auth.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add standard Azure response headers
		w.Header().Set("x-ms-version", APIVersion)
		w.Header().Set("x-ms-request-id", uuid.New().String())
		w.Header().Set("Server", "Azure-Storage-Local/1.0")

		if clientReqID := r.Header.Get("x-ms-client-request-id"); clientReqID != "" {
			w.Header().Set("x-ms-client-request-id", clientReqID)
		}

		next.ServeHTTP(w, r)
	})
}
