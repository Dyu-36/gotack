package main

import (
	"path/filepath"
	"testing"
)

func TestResolveRootPriority(t *testing.T) {
	envRoot := filepath.Join(t.TempDir(), "from-env")
	t.Setenv(rootEnv, envRoot)
	if got := resolveRoot(""); got != envRoot {
		t.Fatalf("environment root = %q", got)
	}
	explicit := filepath.Join(t.TempDir(), "explicit")
	if got := resolveRoot(explicit); got != explicit {
		t.Fatalf("explicit root = %q", got)
	}
}

func TestResolveRootHasAppDefault(t *testing.T) {
	t.Setenv(rootEnv, "")
	if got := resolveRoot(""); filepath.Base(got) != "skills" {
		t.Fatalf("default root = %q", got)
	}
}
