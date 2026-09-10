package semver

import "strings"

// Compare orders semantic versions and returns a negative value, zero, or a
// positive value. Inputs are expected to have been validated by their owning
// contract; trimming whitespace and a leading v keeps build metadata inputs
// from operational version sources comparable.
func Compare(left, right string) int {
	leftCore, leftPrerelease := parts(left)
	rightCore, rightPrerelease := parts(right)
	for index := range leftCore {
		if compared := compareDecimalIdentifier(leftCore[index], rightCore[index]); compared != 0 {
			return compared
		}
	}
	if len(leftPrerelease) == 0 && len(rightPrerelease) > 0 {
		return 1
	}
	if len(leftPrerelease) > 0 && len(rightPrerelease) == 0 {
		return -1
	}
	for index := 0; index < len(leftPrerelease) && index < len(rightPrerelease); index++ {
		leftNumeric := isDecimalIdentifier(leftPrerelease[index])
		rightNumeric := isDecimalIdentifier(rightPrerelease[index])
		switch {
		case leftNumeric && rightNumeric:
			if compared := compareDecimalIdentifier(leftPrerelease[index], rightPrerelease[index]); compared != 0 {
				return compared
			}
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		default:
			if compared := strings.Compare(leftPrerelease[index], rightPrerelease[index]); compared != 0 {
				return compared
			}
		}
	}
	return len(leftPrerelease) - len(rightPrerelease)
}

func parts(value string) ([3]string, []string) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	value = strings.SplitN(value, "+", 2)[0]
	base, prerelease, hasPrerelease := strings.Cut(value, "-")
	segments := strings.Split(base, ".")
	result := [3]string{"0", "0", "0"}
	for index := range result {
		if index < len(segments) && isDecimalIdentifier(segments[index]) {
			result[index] = segments[index]
		}
	}
	if !hasPrerelease || prerelease == "" {
		return result, nil
	}
	return result, strings.Split(prerelease, ".")
}

func isDecimalIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func compareDecimalIdentifier(left, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if left == "" {
		left = "0"
	}
	if right == "" {
		right = "0"
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return strings.Compare(left, right)
}
