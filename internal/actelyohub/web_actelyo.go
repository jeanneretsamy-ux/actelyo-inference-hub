package actelyohub

import (
	"net/http"
	"os"
	"strings"
)

// Permit the ERP to manage a local hub only with an explicitly configured key.
// Never reflect arbitrary origins or allow credentialed wildcard access.
func actelyoCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || origin == "http://"+r.Host || origin == "https://"+r.Host {
			next.ServeHTTP(w, r)
			return
		}
		allowed := false
		origins := os.Getenv("ACTELYO_HUB_ALLOWED_ORIGINS")
		if origins == "" {
			origins = "https://actelyo.com,https://www.actelyo.com"
		}
		for _, candidate := range strings.Split(origins, ",") {
			if origin == strings.TrimSpace(candidate) && origin != "null" && origin != "*" {
				allowed = true
			}
		}
		w.Header().Add("Vary", "Origin")
		if !allowed || !webKeyConfigured() {
			http.Error(w, "ERP origin denied or management key missing", http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
			w.Header().Set("Access-Control-Allow-Private-Network", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
