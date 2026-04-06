package main

import "testing"

func TestParseTUIFilterArgs(t *testing.T) {
	f, err := parseTUIFilterArgs([]string{"--running", "--hours", "6", "--under", "/data", "--match", "ppo"})
	if err != nil {
		t.Fatal(err)
	}
	if f.RunningOnly == nil || !*f.RunningOnly {
		t.Fatalf("expected runningOnly=true")
	}
	if f.Hours != 6 || f.Under != "/data" || f.Match != "ppo" {
		t.Fatalf("unexpected filter: %+v", f)
	}
}

func TestParseTUIFilterArgsConflict(t *testing.T) {
	_, err := parseTUIFilterArgs([]string{"--running", "--not-running"})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}
