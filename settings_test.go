package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPasswordSettingsLifecycle(t *testing.T) {
	db := InitDB(filepath.Join(t.TempDir(), "settings.db"))
	defer db.Close()
	auth := newAuthManager(testPassword)
	if err := auth.loadSettings(db); err != nil {
		t.Fatal(err)
	}
	cookie, csrf := loginForTest(t, auth, "pool.example")
	other, _ := loginForTest(t, auth, "pool.example")
	readPIN := func(cookie *http.Cookie, status int, expected string) {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/settings", nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res := httptest.NewRecorder()
		auth.withSession(auth.settings)(res, req)
		if res.Code != status {
			t.Fatalf("settings status: got %d want %d", res.Code, status)
		}
		if res.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("PIN response can be cached")
		}
		if status != 200 {
			return
		}
		var data struct {
			PIN string `json:"pin"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if data.PIN != expected {
			t.Fatal("settings returned an incorrect PIN")
		}
	}
	readPIN(nil, 401, "")
	readPIN(cookie, 200, testPassword)
	change := func(body, token string) int {
		t.Helper()
		req := httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", token)
		req.AddCookie(cookie)
		res := httptest.NewRecorder()
		auth.withSession(auth.settings)(res, req)
		return res.Code
	}
	disabled := `{"password_enabled":false,"current_password":"` + testPassword + `"}`
	if got := change(disabled, ""); got != 403 {
		t.Fatalf("missing CSRF: %d", got)
	}
	if got := change(`{"password_enabled":false,"current_password":"wrong"}`, csrf); got != 403 {
		t.Fatalf("wrong password: %d", got)
	}
	if got := change(`{"password_enabled":true,"new_password":"short"}`, csrf); got != 400 {
		t.Fatalf("short password: %d", got)
	}
	if got := change(disabled, csrf); got != 204 {
		t.Fatalf("disable: %d", got)
	}
	readPIN(cookie, 200, "")
	readPIN(other, 401, "")
	req := httptest.NewRequest("GET", "/api/pool", nil)
	req.AddCookie(other)
	if _, _, ok := auth.currentSession(req); ok {
		t.Fatal("other session survived settings change")
	}
	restored := newAuthManager(testPassword)
	if err := restored.loadSettings(db); err != nil {
		t.Fatal(err)
	}
	if restored.passwordEnabled {
		t.Fatal("disabled setting did not persist")
	}
	auth = restored
	readPIN(cookie, 401, "")
	res := httptest.NewRecorder()
	auth.sessionInfo(res, httptest.NewRequest("GET", "/api/session", nil))
	if res.Code != 200 || len(res.Result().Cookies()) != 1 {
		t.Fatalf("open session: %d", res.Code)
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("anonymous session can be cached")
	}
	cookie = res.Result().Cookies()[0]
	var session sessionResponse
	if err := json.Unmarshal(res.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	csrf = session.CSRFToken
	readPIN(cookie, 200, "")
	if got := change(`{"password_enabled":true,"new_password":"a different long password"}`, ""); got != 403 {
		t.Fatalf("open session without CSRF: %d", got)
	}
	newPassword := "a different long password"
	if got := change(`{"password_enabled":true,"new_password":"`+newPassword+`"}`, csrf); got != 204 {
		t.Fatalf("enable: %d", got)
	}
	readPIN(cookie, 200, newPassword)
	res = httptest.NewRecorder()
	auth.sessionInfo(res, httptest.NewRequest("GET", "/api/session", nil))
	if res.Code != 401 {
		t.Fatalf("anonymous session after enable: %d", res.Code)
	}
	if got := change(`{"password_enabled":true,"current_password":"`+newPassword+`","new_password":"`+testPassword+`"}`, csrf); got != 204 {
		t.Fatalf("change: %d", got)
	}
	readPIN(cookie, 200, testPassword)
	auth.now = func() time.Time { return time.Now().Add(sessionDuration) }
	readPIN(cookie, 401, "")
	restored = newAuthManager("ignored bootstrap password")
	if err := restored.loadSettings(db); err != nil {
		t.Fatal(err)
	}
	loginForTest(t, restored, "pool.example")
	req = httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"password":"`+newPassword+`"}`))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	restored.login(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("old password accepted: %d", res.Code)
	}
}
