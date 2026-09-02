package attachments

import "testing"

func TestBaseNameAcceptsEitherPathSeparator(t *testing.T) {
	tests := map[string]string{
		`C:\tmp\photo.png`: "photo.png",
		`/tmp/photo.png`:   "photo.png",
		`photo.png`:        "photo.png",
		`  `:               "",
	}
	for input, want := range tests {
		if got := BaseName(input); got != want {
			t.Errorf("BaseName(%q) = %q, want %q", input, got, want)
		}
	}
}
