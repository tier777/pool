package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPasswordPrefersFileAndValidatesPIN(t *testing.T) {
	t.Setenv("APP_PASSWORD", "this environment password is long enough")
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("file password is long enough\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_PASSWORD_FILE", path)

	password, err := loadPassword()
	if err != nil || password != "file password is long enough" {
		t.Fatalf("got password %q and error %v", password, err)
	}

	t.Setenv("APP_PASSWORD_FILE", "")
	t.Setenv("APP_PASSWORD", "123")
	if password, err := loadPassword(); err != nil || password != "123" {
		t.Fatal("three-character PIN was rejected")
	}
	t.Setenv("APP_PASSWORD", "12")
	if _, err := loadPassword(); err == nil {
		t.Fatal("short password was accepted")
	}
}
