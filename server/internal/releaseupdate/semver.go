package releaseupdate

import (
	"fmt"
	"strconv"
	"strings"
)

type semanticVersion struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease []string
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	if value != strings.TrimSpace(value) {
		return semanticVersion{}, fmt.Errorf("version must not contain surrounding whitespace")
	}
	if strings.HasPrefix(value, "v") || strings.HasPrefix(value, "V") {
		return semanticVersion{}, fmt.Errorf("version must not contain a v prefix")
	}
	if buildIndex := strings.IndexByte(value, '+'); buildIndex >= 0 {
		buildMetadata := value[buildIndex+1:]
		if strings.ContainsRune(buildMetadata, '+') || !validSemanticIdentifiers(buildMetadata, false) {
			return semanticVersion{}, fmt.Errorf("invalid build metadata")
		}
		value = value[:buildIndex]
	}
	var prerelease []string
	if index := strings.IndexByte(value, '-'); index >= 0 {
		if index == len(value)-1 {
			return semanticVersion{}, fmt.Errorf("empty prerelease")
		}
		prerelease = strings.Split(value[index+1:], ".")
		value = value[:index]
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semanticVersion{}, fmt.Errorf("version must contain major.minor.patch")
	}
	values := make([]uint64, 3)
	for index, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return semanticVersion{}, fmt.Errorf("invalid numeric version component")
		}
		parsed, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return semanticVersion{}, fmt.Errorf("parse version component: %w", err)
		}
		values[index] = parsed
	}
	if len(prerelease) > 0 && !validSemanticIdentifiers(strings.Join(prerelease, "."), true) {
		return semanticVersion{}, fmt.Errorf("invalid prerelease identifier")
	}
	return semanticVersion{major: values[0], minor: values[1], patch: values[2], prerelease: prerelease}, nil
}

func validSemanticIdentifiers(value string, rejectNumericLeadingZero bool) bool {
	if value == "" {
		return false
	}
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		for _, character := range identifier {
			if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-') {
				return false
			}
		}
		if rejectNumericLeadingZero && isNumericIdentifier(identifier) && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func compareSemanticVersions(left, right string) (int, error) {
	a, err := parseSemanticVersion(left)
	if err != nil {
		return 0, err
	}
	b, err := parseSemanticVersion(right)
	if err != nil {
		return 0, err
	}
	for _, pair := range [][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1, nil
		}
		if pair[0] > pair[1] {
			return 1, nil
		}
	}
	if len(a.prerelease) == 0 && len(b.prerelease) == 0 {
		return 0, nil
	}
	if len(a.prerelease) == 0 {
		return 1, nil
	}
	if len(b.prerelease) == 0 {
		return -1, nil
	}
	limit := min(len(a.prerelease), len(b.prerelease))
	for index := 0; index < limit; index++ {
		leftID, rightID := a.prerelease[index], b.prerelease[index]
		leftNumeric, rightNumeric := isNumericIdentifier(leftID), isNumericIdentifier(rightID)
		switch {
		case leftNumeric && rightNumeric:
			leftValue, _ := strconv.ParseUint(leftID, 10, 64)
			rightValue, _ := strconv.ParseUint(rightID, 10, 64)
			if leftValue < rightValue {
				return -1, nil
			}
			if leftValue > rightValue {
				return 1, nil
			}
		case leftNumeric:
			return -1, nil
		case rightNumeric:
			return 1, nil
		default:
			if leftID < rightID {
				return -1, nil
			}
			if leftID > rightID {
				return 1, nil
			}
		}
	}
	if len(a.prerelease) < len(b.prerelease) {
		return -1, nil
	}
	if len(a.prerelease) > len(b.prerelease) {
		return 1, nil
	}
	return 0, nil
}

func isNumericIdentifier(value string) bool {
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
