//nolint:godoclint,mnd // Private SemVer grammar helpers keep the three core identifiers explicit.
package schematest

import (
	"errors"
	"strings"
)

// semanticVersion is the feature-set portion needed for OAS admission.
type semanticVersion struct {
	major string
	minor string
}

func parseSemanticVersion(source string) (semanticVersion, error) {
	coreAndPrerelease, build, hasBuild := strings.Cut(source, "+")
	if hasBuild {
		if strings.Contains(build, "+") || !validSemVerIdentifiers(build, false) {
			return semanticVersion{}, errors.New("invalid build metadata")
		}
	}

	core, prerelease, hasPrerelease := strings.Cut(coreAndPrerelease, "-")
	if hasPrerelease && !validSemVerIdentifiers(prerelease, true) {
		return semanticVersion{}, errors.New("invalid prerelease")
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semanticVersion{}, errors.New("version core must have major, minor, and patch")
	}

	for _, part := range parts {
		if !validSemVerCoreNumber(part) {
			return semanticVersion{}, errors.New("invalid numeric version identifier")
		}
	}

	return semanticVersion{major: parts[0], minor: parts[1]}, nil
}

func validSemVerCoreNumber(identifier string) bool {
	if identifier == "" || (len(identifier) > 1 && identifier[0] == '0') {
		return false
	}

	for index := range len(identifier) {
		if identifier[index] < '0' || identifier[index] > '9' {
			return false
		}
	}

	return true
}

func validSemVerIdentifiers(source string, prerelease bool) bool {
	if source == "" {
		return false
	}

	for _, identifier := range strings.Split(source, ".") {
		if identifier == "" || !validSemVerIdentifier(identifier) {
			return false
		}

		if prerelease && len(identifier) > 1 && identifier[0] == '0' && identifierIsNumeric(identifier) {
			return false
		}
	}

	return true
}

func validSemVerIdentifier(identifier string) bool {
	for index := range len(identifier) {
		character := identifier[index]
		if (character >= '0' && character <= '9') || (character >= 'A' && character <= 'Z') ||
			(character >= 'a' && character <= 'z') || character == '-' {
			continue
		}

		return false
	}

	return true
}

func identifierIsNumeric(identifier string) bool {
	for index := range len(identifier) {
		if identifier[index] < '0' || identifier[index] > '9' {
			return false
		}
	}

	return true
}
