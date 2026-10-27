package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestWebGuard(t *testing.T) {
	s := newWebServer("127.0.0.1:4747")
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := s.guard(ok)

	cases := []struct {
		name, method, host, origin, ctype string
		want                              int
	}{
		{"page load", "GET", "127.0.0.1:4747", "", "", http.StatusTeapot},
		{"localhost alias", "GET", "localhost:4747", "", "", http.StatusTeapot},
		{"dns rebinding host", "GET", "evil.example:4747", "", "", http.StatusForbidden},
		{"wrong port", "GET", "127.0.0.1:9999", "", "", http.StatusForbidden},
		{"same-origin mutation", "POST", "127.0.0.1:4747", "http://127.0.0.1:4747", "application/json", http.StatusTeapot},
		{"cross-origin mutation", "POST", "127.0.0.1:4747", "https://evil.example", "application/json", http.StatusForbidden},
		{"mutation without origin", "POST", "127.0.0.1:4747", "", "application/json", http.StatusForbidden},
		{"form post from page", "POST", "127.0.0.1:4747", "http://127.0.0.1:4747", "text/plain", http.StatusUnsupportedMediaType},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "/api/run", strings.NewReader("{}"))
			req.Host = c.host
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			if c.ctype != "" {
				req.Header.Set("Content-Type", c.ctype)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Errorf("got %d, want %d", rec.Code, c.want)
			}
		})
	}
}

func TestParsePackageRef(t *testing.T) {
	for _, name := range []string{"jq", "python@3.12", "hashicorp/tap/terraform", "gtk+3", "visual-studio-code"} {
		if err := parsePackageRef("formula", name); err != nil {
			t.Errorf("%q rejected: %v", name, err)
		}
	}
	for _, name := range []string{"", "--force", "-v", "jq; rm -rf /", "a b", "../etc"} {
		if err := parsePackageRef("formula", name); err == nil {
			t.Errorf("%q accepted", name)
		}
	}
	if err := parsePackageRef("tap", "jq"); err == nil {
		t.Error("unknown type accepted")
	}
}

func TestBrewArgsDisambiguatesInstall(t *testing.T) {
	cases := []struct {
		action, typ string
		want        []string
	}{
		{"install", "formula", []string{"install", "--formula", "docker"}},
		{"install", "cask", []string{"install", "--cask", "docker"}},
		{"uninstall", "formula", []string{"uninstall", "docker"}},
		{"upgrade", "cask", []string{"upgrade", "--cask", "docker"}},
	}
	for _, c := range cases {
		if got := brewArgs(c.action, c.typ, "docker"); !slices.Equal(got, c.want) {
			t.Errorf("brewArgs(%s, %s) = %v, want %v", c.action, c.typ, got, c.want)
		}
	}
}
