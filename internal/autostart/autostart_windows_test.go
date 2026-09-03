//go:build windows

package autostart

import "testing"

func TestCommandLineQuotesPathAndStartsHidden(t *testing.T) {
	got := commandLine(`C:\Program Files\Tack\tack.exe`)
	want := `"C:\Program Files\Tack\tack.exe" --hidden`
	if got != want {
		t.Fatalf("commandLine = %q, want %q", got, want)
	}
}
