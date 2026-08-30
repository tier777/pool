package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithAuthLocksOutRepeatedFailures(t *testing.T) {
	t.Setenv("APP_PASSWORD", "correct horse battery staple")
	handler := withAuth(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	for i := 0; i < maxAuthFailures; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/pool", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("Authorization", "Bearer wrong")
		res := httptest.NewRecorder()
		handler(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, res.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/pool", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Authorization", "Bearer correct horse battery staple")
	res := httptest.NewRecorder()
	handler(res, req)
	if res.Code != http.StatusTooManyRequests || res.Header().Get("Retry-After") == "" {
		t.Fatalf("got status %d and Retry-After %q", res.Code, res.Header().Get("Retry-After"))
	}
}

func TestSavePoolRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		want        int
	}{
		{name: "wrong content type", contentType: "text/plain", body: `{"content":"ok"}`, want: http.StatusUnsupportedMediaType},
		{name: "trailing data", contentType: "application/json", body: `{"content":"ok"} trailing`, want: http.StatusBadRequest},
		{name: "unknown field", contentType: "application/json", body: `{"content":"ok","extra":true}`, want: http.StatusBadRequest},
		{name: "too large", contentType: "application/json", body: `{"content":"` + strings.Repeat("x", maxContentBytes) + `"}`, want: http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/pool", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			res := httptest.NewRecorder()
			SavePoolHandler(nil)(res, req)
			if res.Code != tt.want {
				t.Fatalf("got %d, want %d", res.Code, tt.want)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))

	for _, name := range []string{
		"Cross-Origin-Opener-Policy",
		"Permissions-Policy",
		"Referrer-Policy",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-Robots-Tag",
	} {
		if res.Header().Get(name) == "" {
			t.Errorf("missing %s", name)
		}
	}
}
