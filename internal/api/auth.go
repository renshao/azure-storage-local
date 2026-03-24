package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	AccountName = "devstoreaccount1"
	AccountKey  = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	APIVersion  = "2024-08-04"
)

// AuthMiddleware adds standard Azure response headers and validates SAS token expiry.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add standard Azure response headers
		w.Header().Set("x-ms-version", APIVersion)
		w.Header().Set("x-ms-request-id", uuid.New().String())
		w.Header().Set("Server", "Azure-Storage-Local/1.0")

		if clientReqID := r.Header.Get("x-ms-client-request-id"); clientReqID != "" {
			w.Header().Set("x-ms-client-request-id", clientReqID)
		}

		// Check SAS token expiry if present
		sig := r.URL.Query().Get("sig")
		se := r.URL.Query().Get("se")
		if sig != "" && se != "" {
			// Try parsing the signed expiry (ISO 8601 format)
			expiry, err := time.Parse("2006-01-02T15:04:05Z", se)
			if err != nil {
				// Try alternate format without seconds
				expiry, err = time.Parse("2006-01-02T15:04Z", se)
			}
			if err == nil && time.Now().UTC().After(expiry) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusForbidden)
				resp := ErrorResponse{
					Code:    "AuthenticationFailed",
					Message: fmt.Sprintf("SAS token has expired. Signed expiry: %s", se),
				}
				data, _ := xml.MarshalIndent(resp, "", "  ")
				fmt.Fprintf(w, "%s%s", xml.Header, string(data))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
