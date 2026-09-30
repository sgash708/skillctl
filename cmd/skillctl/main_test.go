package main

import "testing"

func TestNewRootCmd(t *testing.T) {
	cmd := newRootCmd()
	if cmd.Use != "skillctl" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "skillctl")
	}
	if cmd.Version == "" {
		t.Error("Version is empty")
	}
}
