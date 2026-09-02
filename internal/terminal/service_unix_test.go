//go:build !windows

package terminal

import (
	"os"
	"strings"
	"testing"
)

func TestPickShellPrefersEnv(t *testing.T) {
	dir := t.TempDir()
	fakereal := dir + "/myshell"
	if err := writeExecutable(t, fakereal); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", fakereal)

	got, args := pickShell()
	if got != fakereal {
		t.Fatalf("pickShell: got %q, want %q", got, fakereal)
	}
	if len(args) == 0 {
		t.Fatalf("pickShell: empty args")
	}
}

func TestPickShellFallsBackWhenEnvMissing(t *testing.T) {

	t.Setenv("SHELL", "/this/path/does/not/exist")

	got, _ := pickShell()
	if got != "/bin/bash" && got != "/bin/sh" {
		t.Fatalf("pickShell: want /bin/bash or /bin/sh, got %q", got)
	}
}

func TestPickShellIgnoresEmptyEnv(t *testing.T) {
	t.Setenv("SHELL", "   ")
	got, _ := pickShell()
	if got != "/bin/bash" && got != "/bin/sh" {
		t.Fatalf("pickShell: want /bin/bash or /bin/sh, got %q", got)
	}
}

func TestWithTERMSetsAndReplaces(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{
			name: "no TERM",
			in:   []string{"PATH=/usr/bin", "HOME=/root"},
			want: "xterm-256color",
		},
		{
			name: "TERM=dumb",
			in:   []string{"PATH=/usr/bin", "TERM=dumb"},
			want: "xterm-256color",
		},
		{
			name: "TERM already good",
			in:   []string{"TERM=xterm-256color", "PATH=/usr/bin"},
			want: "xterm-256color",
		},
		{
			name: "TERM-lookalike kept",
			in:   []string{"TERM_FOO=bar"},
			want: "xterm-256color",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := withTERM(c.in)
			var seen string
			count := 0
			for _, kv := range out {
				if strings.HasPrefix(kv, "TERM=") {
					seen = kv[len("TERM="):]
					count++
				}
			}
			if count != 1 {
				t.Fatalf("expected exactly one TERM entry, got %d in %#v", count, out)
			}
			if seen != c.want {
				t.Fatalf("TERM=%q, want %q (out=%#v)", seen, c.want, out)
			}
		})
	}
}

func writeExecutable(t *testing.T, path string) error {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}
