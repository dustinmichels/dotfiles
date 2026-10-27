package main

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFiles embed.FS

// Package names as Homebrew spells them: plain ("jq"), versioned ("python@3.12"),
// tap-qualified ("hashicorp/tap/terraform"), or with "+" ("gtk+3").
// A leading "-" is rejected so a name can never be read as a brew flag.
var validPackageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9@+._/-]{0,199}$`)

var webActions = map[string]bool{"install": true, "upgrade": true, "uninstall": true}

type webServer struct {
	// allowedHosts holds the Host header values that may reach the server.
	// Anything else is rejected to defeat DNS-rebinding attacks.
	allowedHosts map[string]bool
	// brewMu serializes mutating brew commands; brew holds its own locks and
	// concurrent installs would just fail.
	brewMu sync.Mutex
}

func newWebServer(listenAddr string) *webServer {
	_, port, _ := net.SplitHostPort(listenAddr)
	return &webServer{allowedHosts: map[string]bool{
		"127.0.0.1:" + port: true,
		"localhost:" + port: true,
	}}
}

func runWeb(port int, openBrowser bool) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	s := newWebServer(ln.Addr().String())
	url := "http://" + ln.Addr().String()
	fmt.Printf("brew-ls web UI running at %s (Ctrl+C to stop)\n", url)
	if openBrowser {
		_ = exec.Command("open", url).Start()
	}
	srv := &http.Server{Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ln)
}

func (s *webServer) routes() http.Handler {
	static, _ := fs.Sub(webFiles, "web")
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/packages", s.handlePackages)
	mux.HandleFunc("GET /api/outdated", s.handleOutdated)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("POST /api/run", s.handleRun)
	return s.guard(mux)
}

// guard rejects requests that did not come from the brew-ls page itself:
// unknown Host headers (DNS rebinding) and cross-origin or non-JSON mutations (CSRF).
func (s *webServer) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowedHosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("Origin") != "http://"+r.Host {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// brewQuery runs a read-only brew command without triggering Homebrew's auto-update.
func brewQuery(args ...string) (stdout []byte, stderr string, err error) {
	cmd := exec.Command("brew", args...)
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1")
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	stderr = strings.TrimSpace(errBuf.String())
	if err != nil && stderr != "" {
		err = fmt.Errorf("brew %s: %s", args[0], stderr)
	}
	return out, stderr, err
}

func parsePackageRef(typ, name string) error {
	if typ != "formula" && typ != "cask" {
		return fmt.Errorf("type must be \"formula\" or \"cask\", got %q", typ)
	}
	if !validPackageName.MatchString(name) {
		return fmt.Errorf("invalid package name %q", name)
	}
	return nil
}

func (s *webServer) handlePackages(w http.ResponseWriter, r *http.Request) {
	items, err := loadPackages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if items == nil {
		items = []PackageItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

type outdatedEntry struct {
	Installed string `json:"installed"`
	Latest    string `json:"latest"`
	Pinned    bool   `json:"pinned"`
}

func (s *webServer) handleOutdated(w http.ResponseWriter, r *http.Request) {
	out, _, err := brewQuery("outdated", "--json=v2")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var raw struct {
		Formulae []struct {
			Name              string   `json:"name"`
			InstalledVersions []string `json:"installed_versions"`
			CurrentVersion    string   `json:"current_version"`
			Pinned            bool     `json:"pinned"`
		} `json:"formulae"`
		Casks []struct {
			Name              string   `json:"name"`
			InstalledVersions []string `json:"installed_versions"`
			CurrentVersion    string   `json:"current_version"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to parse brew outdated json: %w", err))
		return
	}
	result := map[string]map[string]outdatedEntry{"formula": {}, "cask": {}}
	for _, f := range raw.Formulae {
		result["formula"][f.Name] = outdatedEntry{strings.Join(f.InstalledVersions, ", "), f.CurrentVersion, f.Pinned}
	}
	for _, c := range raw.Casks {
		result["cask"][c.Name] = outdatedEntry{strings.Join(c.InstalledVersions, ", "), c.CurrentVersion, false}
	}
	writeJSON(w, http.StatusOK, result)
}

// handleInfo returns Homebrew's own JSON for one formula or cask, installed or not.
func (s *webServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	typ, name := r.URL.Query().Get("type"), r.URL.Query().Get("name")
	if err := parsePackageRef(typ, name); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	out, _, err := brewQuery("info", "--json=v2", "--"+typ, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	var raw struct {
		Formulae []json.RawMessage `json:"formulae"`
		Casks    []json.RawMessage `json:"casks"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to parse brew info json: %w", err))
		return
	}
	list := raw.Formulae
	if typ == "cask" {
		list = raw.Casks
	}
	if len(list) == 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("no %s named %q", typ, name))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(list[0])
}

func (s *webServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || strings.HasPrefix(q, "-") || len(q) > 100 {
		writeError(w, http.StatusBadRequest, errors.New("search text must be 1–100 characters and not start with \"-\""))
		return
	}
	result := map[string][]string{"formula": {}, "cask": {}}
	for _, typ := range []string{"formula", "cask"} {
		out, stderr, err := brewQuery("search", "--"+typ, q)
		if err != nil {
			// brew exits 1 when nothing matches; that is an empty result, not a failure.
			if strings.Contains(stderr, "No formulae or casks found") {
				continue
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, line := range strings.Split(string(out), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "==>") {
				result[typ] = append(result[typ], line)
			}
		}
	}
	writeJSON(w, http.StatusOK, result)
}

type runRequest struct {
	Action string `json:"action"`
	Type   string `json:"type"`
	Name   string `json:"name"`
}

// runEvent is one NDJSON line of a streamed brew command.
// Output lines carry Line; the final event has Done set.
type runEvent struct {
	Line  string `json:"line,omitempty"`
	Done  bool   `json:"done,omitempty"`
	OK    bool   `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *webServer) handleRun(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return
	}
	if !webActions[req.Action] {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown action %q", req.Action))
		return
	}
	if err := parsePackageRef(req.Type, req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.brewMu.TryLock() {
		writeError(w, http.StatusConflict, errors.New("another brew command is still running"))
		return
	}
	defer s.brewMu.Unlock()

	args := brewArgs(req.Action, req.Type, req.Name)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	send := func(ev runEvent) {
		// Write errors mean the tab went away; brew keeps running to completion regardless.
		_ = enc.Encode(ev)
		if flusher != nil {
			flusher.Flush()
		}
	}

	send(runEvent{Line: "$ brew " + strings.Join(args, " ")})

	// Not tied to the request context: closing the tab must not kill brew mid-install.
	cmd := exec.Command("brew", args...)
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_ENV_HINTS=1", "HOMEBREW_NO_COLOR=1")
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		send(runEvent{Done: true, Error: err.Error()})
		return
	}
	waitErr := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		pw.Close()
		waitErr <- err
	}()

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		// Progress bars redraw with \r; keep only the final frame.
		if i := strings.LastIndexByte(strings.TrimRight(line, "\r"), '\r'); i >= 0 {
			line = line[i+1:]
		}
		send(runEvent{Line: strings.TrimRight(line, "\r")})
	}
	_, _ = io.Copy(io.Discard, pr) // keep draining if the scanner stopped early

	if err := <-waitErr; err != nil {
		send(runEvent{Done: true, Error: fmt.Sprintf("brew %s failed: %v", req.Action, err)})
		return
	}
	send(runEvent{Done: true, OK: true})
}
