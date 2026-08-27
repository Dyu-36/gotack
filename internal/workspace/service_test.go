package workspace

import (
	"runtime"
	"testing"
)

// samePathTestCase captures both the platform-agnostic identity rule
// (cleaned, absolute) and the platform-specific case-sensitivity rule.
type samePathTestCase struct {
	name string
	a, b string
	want bool
}

func TestSamePathPlatformConvention(t *testing.T) {
	cases := []samePathTestCase{
		{name: "identical", a: "/tmp/proj", b: "/tmp/proj", want: true},
		{name: "empty", a: "", b: "/tmp/proj", want: false},
		{name: "trailing slash cleaned", a: "/tmp/proj", b: "/tmp/proj/", want: true},
		{name: "double slash collapsed", a: "/tmp/proj", b: "/tmp//proj", want: true},
		{name: "dot segment cleaned", a: "/tmp/proj", b: "/tmp/./proj", want: true},
		{name: "different dir", a: "/tmp/proj", b: "/tmp/other", want: false},
	}
	// Case sensitivity only matters on Windows and macOS.
	caseInsensitive := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	if caseInsensitive {
		cases = append(cases,
			samePathTestCase{name: "case differs windows fs", a: "C:\\Proj", b: "c:\\proj", want: true},
		)
	} else {
		cases = append(cases,
			samePathTestCase{name: "case differs posix fs", a: "/tmp/Proj", b: "/tmp/proj", want: false},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := samePath(tc.a, tc.b); got != tc.want {
				t.Fatalf("samePath(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
