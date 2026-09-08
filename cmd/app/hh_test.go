package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func TestHHStartRequiresCredentials(t *testing.T) {
	hh := newHHClient(config{}, http.DefaultClient)
	recorder := httptest.NewRecorder()
	hh.handleStart(recorder, httptest.NewRequest(http.MethodGet, "/auth/hh/start", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestHHStartCreatesBoundState(t *testing.T) {
	hh := newHHClient(config{HHClientID: "client", HHClientSecret: "secret", HHRedirectURL: "http://localhost:8080/auth/hh/callback"}, http.DefaultClient)
	recorder := httptest.NewRecorder()
	hh.handleStart(recorder, httptest.NewRequest(http.MethodGet, "/auth/hh/start", nil))

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusFound)
	}
	redirect, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := redirect.Query().Get("state")
	if state == "" || redirect.Query().Get("redirect_uri") != hh.cfg.HHRedirectURL {
		t.Fatalf("unexpected OAuth redirect: %s", redirect)
	}
	if len(recorder.Result().Cookies()) != 1 || recorder.Result().Cookies()[0].Value != state {
		t.Fatal("OAuth state is not bound to a browser cookie")
	}
}

func TestTokenStoreUsesOwnerOnlyPermissions(t *testing.T) {
	hh := newHHClient(config{LocalStoragePath: t.TempDir()}, http.DefaultClient)
	want := hhToken{AccessToken: "access", RefreshToken: "refresh"}
	if err := hh.saveToken(want); err != nil {
		t.Fatal(err)
	}
	got, err := hh.loadToken()
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Fatalf("token = %#v, want %#v", got, want)
	}
	info, err := os.Stat(hh.tokenPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o, want 600", info.Mode().Perm())
	}
}
