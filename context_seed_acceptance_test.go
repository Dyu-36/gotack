package main

import (
	"context"
	"github.com/Dyu-36/gotack/internal/contextseed"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestContextReconnectFailureKeepsServerLease(t *testing.T) {
	for _, failure := range []string{"build", "set", "remove", "case-alias", "relative-profile", "separator-alias"} {
		t.Run(failure, func(t *testing.T) {
			if failure == "case-alias" && runtime.GOOS != "windows" {
				t.Skip("Windows case alias")
			}
			data := t.TempDir()
			s := contextseed.New(data, nil)
			if err := os.MkdirAll(s.ContextDir(), 0755); err != nil {
				t.Fatal(err)
			}
			core := filepath.Join(s.ContextDir(), "TACK_CORE.md")
			write := func(body string) {
				t.Helper()
				if err := os.WriteFile(core, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("generation one")
			fake := &contextRegistrationAPI{t: t}
			first := newContextLeaseTestApp(t, s, fake)
			first.registerContextPaths("reconnect")
			gen1 := fake.contextPath[0]
			first.releaseAllContextLeases()
			second := newContextLeaseTestApp(t, s, fake)
			if failure == "relative-profile" {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				rel, err := filepath.Rel(cwd, data)
				if err != nil {
					t.Fatal(err)
				}
				second.contextSeeder = contextseed.New(rel, nil)
			}
			switch failure {
			case "build":
				if err := os.WriteFile(filepath.Join(gen1, "TACK_CORE.md"), []byte("corrupt committed"), 0644); err != nil {
					t.Fatal(err)
				}
			case "set":
				write("generation two")
				fake.failNextSet = true
			case "remove":
				fake.failNextRemove = true
			case "relative-profile", "separator-alias":
				fake.contextPath = []string{filepath.ToSlash(gen1)}
				write("generation two")
				fake.failNextRefresh = true
			case "case-alias":
				fake.contextPath = []string{filepath.Join(strings.ToUpper(filepath.Dir(gen1)), filepath.Base(gen1))}
				write("generation two")
				fake.failNextRefresh = true
			}
			if failure == "remove" {
				second.clearContextPath(context.Background(), second.getConn().api, "reconnect")
			} else {
				second.registerContextPaths("reconnect")
			}
			if second.contextLeaseGeneration("reconnect") == "" {
				t.Error("server generation lost protection after reconnect failure")
			}
			write("generation three")
			if _, err := s.BuildPromptSnapshot(); err != nil {
				t.Fatal(err)
			}
			write("generation four")
			gen4, err := s.BuildPromptSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			pruner := contextseed.New(data, nil)
			if err := pruner.PrunePromptSnapshotsChecked(gen4); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(gen1); err != nil {
				t.Fatalf("server-referenced generation pruned after reconnect failure: %v", err)
			}
			second.releaseAllContextLeases()
			if err := pruner.PrunePromptSnapshotsChecked(gen4); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(gen1); !os.IsNotExist(err) {
				t.Fatalf("released generation not prunable: %v", err)
			}
		})
	}
}
