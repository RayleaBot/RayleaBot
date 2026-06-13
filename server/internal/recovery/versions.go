package recovery

import (
	"strconv"
	"strings"
)

func classifyOperation(sourceVersion, targetVersion string) string {
	switch compareSemver(sourceVersion, targetVersion) {
	case -1:
		return "upgrade"
	case 1:
		return "rollback"
	default:
		return "restore"
	}
}

func compareSemver(left, right string) int {
	lp := semverParts(left)
	rp := semverParts(right)
	for i := 0; i < 3; i++ {
		if lp[i] < rp[i] {
			return -1
		}
		if lp[i] > rp[i] {
			return 1
		}
	}
	return 0
}

func semverParts(version string) [3]int {
	var parts [3]int
	if version == "" {
		return parts
	}
	cleaned := version
	for _, marker := range []string{"-", "+"} {
		if idx := strings.Index(cleaned, marker); idx >= 0 {
			cleaned = cleaned[:idx]
		}
	}
	items := strings.Split(cleaned, ".")
	for i := 0; i < len(items) && i < 3; i++ {
		value, err := strconv.Atoi(strings.TrimSpace(items[i]))
		if err == nil {
			parts[i] = value
		}
	}
	return parts
}

func isSchemaNewer(source, target string) bool {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if source == "" || target == "" {
		return false
	}
	left, leftErr := strconv.Atoi(source)
	right, rightErr := strconv.Atoi(target)
	if leftErr == nil && rightErr == nil {
		return left > right
	}
	leftBase, leftBaseOK := baseSchemaVersion(source)
	rightBase, rightBaseOK := baseSchemaVersion(target)
	if leftBaseOK && rightBaseOK {
		return leftBase > rightBase
	}
	if leftBaseOK != rightBaseOK || leftErr != nil || rightErr != nil {
		return false
	}
	return compareSemver(source, target) > 0
}

func baseSchemaVersion(version string) (int, bool) {
	const prefix = "base-"
	if !strings.HasPrefix(version, prefix) {
		return 0, false
	}
	parts := strings.Split(strings.TrimPrefix(version, prefix), "-")
	if len(parts) != 2 {
		return 0, false
	}
	year, yearErr := strconv.Atoi(parts[0])
	month, monthErr := strconv.Atoi(parts[1])
	if yearErr != nil || monthErr != nil || month < 1 || month > 12 {
		return 0, false
	}
	return year*100 + month, true
}
