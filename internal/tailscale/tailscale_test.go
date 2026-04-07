package tailscale

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeExec(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDetectOverride(t *testing.T) {
	oldExec := isExecutableFn
	defer func() { isExecutableFn = oldExec }()
	isExecutableFn = isExecutablePath

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "tailscale")
	makeExec(t, bin)
	d, err := Detect(bin)
	if err != nil {
		t.Fatal(err)
	}
	if d.Path != bin || d.Method != "config_or_env_override" {
		t.Fatalf("unexpected detection: %+v", d)
	}
}

func TestDetectByLookPath(t *testing.T) {
	oldLookPath := lookPathFn
	oldHome := userHomeDirFn
	oldExec := isExecutableFn
	oldCandidates := commonCandidatesFn
	defer func() {
		lookPathFn = oldLookPath
		userHomeDirFn = oldHome
		isExecutableFn = oldExec
		commonCandidatesFn = oldCandidates
	}()
	lookPathFn = func(file string) (string, error) {
		if file == "tailscale" {
			return "/usr/bin/tailscale", nil
		}
		return "", errors.New("not found")
	}
	userHomeDirFn = func() (string, error) { return t.TempDir(), nil }
	commonCandidatesFn = commonCandidates
	isExecutableFn = isExecutablePath
	d, err := Detect("")
	if err != nil {
		t.Fatal(err)
	}
	if d.Method != "path_lookup" {
		t.Fatalf("expected path_lookup got %s", d.Method)
	}
}

func TestDetectCommonUserPathFallback(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin", "tailscale")
	makeExec(t, bin)

	oldLookPath := lookPathFn
	oldHome := userHomeDirFn
	oldExec := isExecutableFn
	oldCandidates := commonCandidatesFn
	defer func() {
		lookPathFn = oldLookPath
		userHomeDirFn = oldHome
		isExecutableFn = oldExec
		commonCandidatesFn = oldCandidates
	}()
	lookPathFn = func(string) (string, error) { return "", errors.New("not found") }
	userHomeDirFn = func() (string, error) { return home, nil }
	commonCandidatesFn = func() []string {
		return []string{
			filepath.Join(home, ".local", "bin", "tailscale"),
			filepath.Join(home, "bin", "tailscale"),
		}
	}
	isExecutableFn = isExecutablePath
	d, err := Detect("")
	if err != nil {
		t.Fatal(err)
	}
	if d.Path != bin {
		t.Fatalf("expected %s got %s", bin, d.Path)
	}
}

func TestDetectNotFound(t *testing.T) {
	oldLookPath := lookPathFn
	oldHome := userHomeDirFn
	oldExec := isExecutableFn
	oldCandidates := commonCandidatesFn
	defer func() {
		lookPathFn = oldLookPath
		userHomeDirFn = oldHome
		isExecutableFn = oldExec
		commonCandidatesFn = oldCandidates
	}()
	lookPathFn = func(string) (string, error) { return "", errors.New("not found") }
	userHomeDirFn = func() (string, error) { return t.TempDir(), nil }
	commonCandidatesFn = func() []string {
		return []string{"/tmp/this/path/does/not/exist/tailscale"}
	}
	isExecutableFn = isExecutablePath
	_, err := Detect("")
	if err == nil {
		t.Fatal("expected error when tailscale not found")
	}
}

func TestParseServeURL(t *testing.T) {
	out := `https://drlserver.tailb5bd65.ts.net (tailnet only)
|-- / proxy http://127.0.0.1:6786
`
	u := ParseServeURL(out)
	if u != "https://drlserver.tailb5bd65.ts.net" {
		t.Fatalf("unexpected url: %s", u)
	}
}

func TestServeOffCommand(t *testing.T) {
	cmd := ServeOffCommand("/usr/bin/tailscale")
	joined := strings.Join(cmd, " ")
	if joined != "/usr/bin/tailscale serve --https=443 off" {
		t.Fatalf("unexpected off command: %s", joined)
	}
}

func TestRunServeTimeoutButServeAvailable(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "tailscale")
	script := `#!/bin/sh
if [ "$1" = "serve" ] && [ "$2" = "status" ]; then
  echo "https://demo.tail.ts.net (tailnet only)"
  echo "|-- / proxy http://127.0.0.1:6786"
  exit 0
fi
if [ "$1" = "serve" ] && [ "$2" = "--bg" ]; then
  sleep 1
  echo "Serve started and running in the background."
  exit 0
fi
exit 1
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	oldServeTimeout := serveTimeout
	oldStatusTimeout := statusTimeout
	defer func() {
		serveTimeout = oldServeTimeout
		statusTimeout = oldStatusTimeout
	}()
	serveTimeout = 50 * time.Millisecond
	statusTimeout = 300 * time.Millisecond

	out, err := RunServe(bin, "http://127.0.0.1:6786")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(out, "tailnet 入口") {
		t.Fatalf("expected timeout recovery hint in output, got: %s", out)
	}
}

func TestRunServeTimeoutAndUnavailable(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "tailscale")
	script := `#!/bin/sh
if [ "$1" = "serve" ] && [ "$2" = "status" ]; then
  echo "No serve config"
  exit 0
fi
if [ "$1" = "serve" ] && [ "$2" = "--bg" ]; then
  sleep 1
  exit 0
fi
exit 1
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	oldServeTimeout := serveTimeout
	oldStatusTimeout := statusTimeout
	defer func() {
		serveTimeout = oldServeTimeout
		statusTimeout = oldStatusTimeout
	}()
	serveTimeout = 50 * time.Millisecond
	statusTimeout = 300 * time.Millisecond

	_, err := RunServe(bin, "http://127.0.0.1:6786")
	if err == nil {
		t.Fatalf("expected error when timeout and no tailnet url")
	}
}
