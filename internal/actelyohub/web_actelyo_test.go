package actelyohub

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActelyoCORS(t *testing.T) {
	testHome(t)
	t.Setenv("ACTELYO_HUB_ALLOWED_ORIGINS", "https://actelyo.com")
	if err := storeWebKey("erp-key"); err != nil {
		t.Fatal(err)
	}
	handler := actelyoCORS(requireWebAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, tc := range []struct {
		name, method, origin, key string
		status                    int
	}{
		{"authorized ERP", "GET", "https://actelyo.com", "erp-key", 200},
		{"missing bearer", "GET", "https://actelyo.com", "", 401},
		{"wrong bearer", "GET", "https://actelyo.com", "wrong", 401},
		{"foreign origin", "GET", "https://untrusted.example", "erp-key", 403},
		{"opaque origin", "GET", "null", "erp-key", 403},
		{"preflight", "OPTIONS", "https://actelyo.com", "", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://127.0.0.1:8090/api/status", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Access-Control-Request-Private-Network", "true")
			if tc.key != "" {
				r.Header.Set("Authorization", "Bearer "+tc.key)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d", w.Code, tc.status)
			}
			if tc.status == 204 && w.Header().Get("Access-Control-Allow-Private-Network") != "true" {
				t.Fatal("private-network preflight missing")
			}
		})
	}
	if err := storeWebKey(""); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "http://127.0.0.1:8090/api/status", nil)
	r.Header.Set("Origin", "https://actelyo.com")
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatal("cross-origin control must require a configured management key")
	}
}
