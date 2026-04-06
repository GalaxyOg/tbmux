package tailscale

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
