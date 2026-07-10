package releaseupdate

import "testing"

func TestSemanticVersionParserMatchesReleaseContract(t *testing.T) {
	for _, value := range []string{"1.2.3", "1.2.3-rc.1", "1.2.3+build.01", "0.0.0"} {
		if _, err := parseSemanticVersion(value); err != nil {
			t.Fatalf("valid version %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"v1.2.3", "1.2", "01.2.3", "1.2.3+", "1.2.3+bad value", "1.2.3-alpha..1", "1.2.3-01"} {
		if _, err := parseSemanticVersion(value); err == nil {
			t.Fatalf("invalid version %q accepted", value)
		}
	}
}

func TestSemanticVersionComparisonIgnoresBuildMetadata(t *testing.T) {
	comparison, err := compareSemanticVersions("1.2.3+build.1", "1.2.3+build.2")
	if err != nil || comparison != 0 {
		t.Fatalf("build metadata changed precedence: %d, %v", comparison, err)
	}
	comparison, err = compareSemanticVersions("1.2.3", "1.2.3-rc.1")
	if err != nil || comparison <= 0 {
		t.Fatalf("final release should be newer than prerelease: %d, %v", comparison, err)
	}
}
