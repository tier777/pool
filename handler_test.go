package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testPassword = "correct horse battery staple"

func loginForTest(t *testing.T, auth *authManager, host string) (*http.Cookie, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "https://"+host+"/api/login", strings.NewReader(`{"password":"`+testPassword+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.1:1234"
	res := httptest.NewRecorder()
	auth.login(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("login got %d: %s", res.Code, res.Body.String())
	}
	cookies := res.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	var body sessionResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return cookies[0], body.CSRFToken
}

func TestLoginCreatesProtectedSessionCookie(t *testing.T) {
	auth := newAuthManager(testPassword)
	cookie, csrf := loginForTest(t, auth, "pool.example")
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie flags: HttpOnly=%v Secure=%v SameSite=%v", cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}
	if cookie.Path != "/" || cookie.MaxAge != int(sessionDuration.Seconds()) {
		t.Fatalf("cookie scope: Path=%q MaxAge=%d", cookie.Path, cookie.MaxAge)
	}
	if csrf == "" || cookie.Value == "" || strings.Contains(cookie.Value, testPassword) {
		t.Fatal("session tokens must be random and non-empty")
	}

	localReq := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/login", strings.NewReader(`{"password":"`+testPassword+`"}`))
	localReq.Header.Set("Content-Type", "application/json")
	localReq.RemoteAddr = "192.0.2.2:1234"
	localRes := httptest.NewRecorder()
	auth.login(localRes, localReq)
	if localRes.Result().Cookies()[0].Secure {
		t.Fatal("loopback HTTP cookie should remain usable without Secure")
	}
}

func TestLoginLocksOutRepeatedFailures(t *testing.T) {
	auth := newAuthManager(testPassword)

	for i := 0; i < maxAuthFailures; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"wrong"}`))
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		auth.login(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, res.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"`+testPassword+`"}`))
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	auth.login(res, req)
	if res.Code != http.StatusTooManyRequests || res.Header().Get("Retry-After") == "" {
		t.Fatalf("got status %d and Retry-After %q", res.Code, res.Header().Get("Retry-After"))
	}
}

func TestSessionRequiresCSRFAndLogoutRevokesIt(t *testing.T) {
	auth := newAuthManager(testPassword)
	cookie, csrf := loginForTest(t, auth, "pool.example")
	protected := auth.withSession(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/pool", nil)
		req.AddCookie(cookie)
		if token != "" {
			req.Header.Set("X-CSRF-Token", token)
		}
		res := httptest.NewRecorder()
		protected(res, req)
		return res
	}
	if got := request("").Code; got != http.StatusForbidden {
		t.Fatalf("without CSRF got %d, want 403", got)
	}
	if got := request(csrf).Code; got != http.StatusNoContent {
		t.Fatalf("with CSRF got %d, want 204", got)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutReq.AddCookie(cookie)
	logoutReq.Header.Set("X-CSRF-Token", csrf)
	logoutRes := httptest.NewRecorder()
	auth.withSession(auth.logout)(logoutRes, logoutReq)
	if logoutRes.Code != http.StatusNoContent || logoutRes.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout got %d with cookie MaxAge %d", logoutRes.Code, logoutRes.Result().Cookies()[0].MaxAge)
	}
	if got := request(csrf).Code; got != http.StatusUnauthorized {
		t.Fatalf("after logout got %d, want 401", got)
	}
}

func TestSessionExpires(t *testing.T) {
	auth := newAuthManager(testPassword)
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	auth.now = func() time.Time { return now }
	cookie, _ := loginForTest(t, auth, "pool.example")
	now = now.Add(sessionDuration)

	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	auth.withSession(auth.sessionInfo)(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expired session got %d, want 401", res.Code)
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
		"Content-Security-Policy",
		"Cross-Origin-Opener-Policy",
		"Cross-Origin-Resource-Policy",
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

func TestFileUploadDownloadAndDelete(t *testing.T) {
	dir := t.TempDir()
	db := InitDB(filepath.Join(dir, "pool.db"))
	t.Cleanup(func() { db.Close() })
	filesDir := filepath.Join(dir, "files")
	if err := os.Mkdir(filesDir, 0o700); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("hello pool"))
	writer.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/files", &body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRes := httptest.NewRecorder()
	FilesHandler(db, filesDir)(uploadRes, uploadReq)
	if uploadRes.Code != http.StatusCreated {
		t.Fatalf("upload got %d: %s", uploadRes.Code, uploadRes.Body.String())
	}
	var uploaded storedFile
	if err := json.NewDecoder(uploadRes.Body).Decode(&uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.Name != "hello.txt" || uploaded.Size != 10 {
		t.Fatalf("uploaded file = %#v", uploaded)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/api/files/"+strconv.FormatInt(uploaded.ID, 10), nil)
	downloadRes := httptest.NewRecorder()
	FileHandler(db, filesDir)(downloadRes, downloadReq)
	if downloadRes.Code != http.StatusOK || downloadRes.Body.String() != "hello pool" {
		t.Fatalf("download got %d: %q", downloadRes.Code, downloadRes.Body.String())
	}
	if disposition := downloadRes.Header().Get("Content-Disposition"); !strings.Contains(disposition, "hello.txt") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, downloadReq.URL.String(), nil)
	deleteRes := httptest.NewRecorder()
	FileHandler(db, filesDir)(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete got %d: %s", deleteRes.Code, deleteRes.Body.String())
	}

	missingRes := httptest.NewRecorder()
	FileHandler(db, filesDir)(missingRes, downloadReq)
	if missingRes.Code != http.StatusNotFound {
		t.Fatalf("deleted download got %d, want 404", missingRes.Code)
	}
}

func TestFileUploadIsNotCappedAtTenMegabytes(t *testing.T) {
	dir := t.TempDir()
	db := InitDB(filepath.Join(dir, "pool.db"))
	t.Cleanup(func() { db.Close() })
	filesDir := filepath.Join(dir, "files")
	if err := os.Mkdir(filesDir, 0o700); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "large.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), 11<<20)); err != nil {
		t.Fatal(err)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	FilesHandler(db, filesDir)(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("11 MB upload got %d: %s", res.Code, res.Body.String())
	}
}
