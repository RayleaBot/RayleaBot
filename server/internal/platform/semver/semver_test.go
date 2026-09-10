package semver

import "testing"

func TestCompare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		left, right string
		want        int
	}{
		{name: "equal", left: "1.2.3", right: "1.2.3", want: 0},
		{name: "major", left: "2.0.0", right: "1.999.999", want: 1},
		{name: "prerelease before release", left: "1.0.0-alpha", right: "1.0.0", want: -1},
		{name: "numeric before alphabetic", left: "1.0.0-alpha.1", right: "1.0.0-alpha.beta", want: -1},
		{name: "prerelease identifiers", left: "1.0.0-beta.11", right: "1.0.0-rc.1", want: -1},
		{name: "shorter prerelease", left: "1.0.0-alpha", right: "1.0.0-alpha.1", want: -1},
		{name: "build metadata ignored", left: "1.0.0+build.2", right: "1.0.0+build.1", want: 0},
		{name: "leading v and whitespace", left: " v1.2.3 ", right: "1.2.3", want: 0},
		{name: "long core number", left: "999999999999999999999999.0.0", right: "2.0.0", want: 1},
		{name: "long prerelease number", left: "1.0.0-999999999999999999999999", right: "1.0.0-2", want: 1},
		{name: "leading zero numeric", left: "1.0.0-01", right: "1.0.0-1", want: 0},
		{name: "blank behaves as zero", left: "", right: "0.0.0", want: 0},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := Compare(testCase.left, testCase.right)
			if got < 0 {
				got = -1
			} else if got > 0 {
				got = 1
			}
			if got != testCase.want {
				t.Fatalf("Compare(%q, %q) = %d, want %d", testCase.left, testCase.right, got, testCase.want)
			}
		})
	}
}
