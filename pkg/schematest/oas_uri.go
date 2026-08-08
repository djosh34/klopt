//nolint:cyclop,godoclint // Private RFC 3986 productions keep component-specific failures explicit.
package schematest

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

type uriReferenceClass uint8

const (
	uriRelativeReference uriReferenceClass = iota
	uriNonRelativeReference
)

func validateURIReference(value string) (uriReferenceClass, error) {
	classification := uriRelativeReference

	reference, fragment, hasFragment := strings.Cut(value, "#")
	if hasFragment {
		if err := validateURIFragment(fragment); err != nil {
			return classification, fmt.Errorf("invalid fragment: %w", err)
		}
	}

	hierarchical, query, hasQuery := strings.Cut(reference, "?")
	if hasQuery {
		if err := validateURIComponent(query, "-._~!$&'()*+,;=:@/?"); err != nil {
			return classification, fmt.Errorf("invalid query: %w", err)
		}
	}

	colon := strings.IndexByte(hierarchical, ':')

	slash := strings.IndexByte(hierarchical, '/')
	if colon >= 0 && (slash < 0 || colon < slash) {
		if !validURIScheme(hierarchical[:colon]) {
			return classification, errors.New("invalid URI scheme")
		}

		classification = uriNonRelativeReference
		hierarchical = hierarchical[colon+1:]
	}

	path := hierarchical
	if strings.HasPrefix(hierarchical, "//") {
		authority := hierarchical[2:]
		if separator := strings.IndexByte(authority, '/'); separator >= 0 {
			path = authority[separator:]
			authority = authority[:separator]
		} else {
			path = ""
		}

		if err := validateURIAuthority(authority); err != nil {
			return classification, fmt.Errorf("invalid authority: %w", err)
		}
	}

	if err := validateURIComponent(path, "-._~!$&'()*+,;=:@/"); err != nil {
		return classification, fmt.Errorf("invalid path: %w", err)
	}

	return classification, nil
}

func validateURIAuthority(authority string) error {
	hostAndPort := authority
	if separator := strings.LastIndexByte(authority, '@'); separator >= 0 {
		userInfo := authority[:separator]
		if strings.ContainsRune(userInfo, '@') {
			return errors.New("multiple user-information delimiters")
		}

		if err := validateURIComponent(userInfo, "-._~!$&'()*+,;=:"); err != nil {
			return fmt.Errorf("invalid user information: %w", err)
		}

		hostAndPort = authority[separator+1:]
	}

	if strings.HasPrefix(hostAndPort, "[") {
		closingBracket := strings.IndexByte(hostAndPort, ']')
		if closingBracket < 0 {
			return errors.New("unterminated IP literal")
		}

		if err := validateIPLiteral(hostAndPort[1:closingBracket]); err != nil {
			return fmt.Errorf("invalid IP literal: %w", err)
		}

		return validateURIPortSuffix(hostAndPort[closingBracket+1:], true)
	}

	if strings.ContainsAny(hostAndPort, "[]") {
		return errors.New("brackets are only allowed around an IP literal")
	}

	host := hostAndPort
	if separator := strings.LastIndexByte(hostAndPort, ':'); separator >= 0 {
		host = hostAndPort[:separator]
		if err := validateURIPortSuffix(hostAndPort[separator:], false); err != nil {
			return err
		}
	}

	if err := validateURIComponent(host, "-._~!$&'()*+,;="); err != nil {
		return fmt.Errorf("invalid host: %w", err)
	}

	return nil
}

func validateIPLiteral(literal string) error {
	if len(literal) > 0 && (literal[0] == 'v' || literal[0] == 'V') {
		return validateIPvFuture(literal)
	}

	address, err := netip.ParseAddr(literal)
	if err != nil || !address.Is6() || address.Zone() != "" {
		return errors.New("must be an IPv6 address or IPvFuture literal")
	}

	return nil
}

func validateIPvFuture(literal string) error {
	version, address, found := strings.Cut(literal[1:], ".")
	if !found || version == "" || address == "" {
		return errors.New("malformed IPvFuture literal")
	}

	for index := 0; index < len(version); index++ {
		if !isHexDigit(version[index]) {
			return errors.New("IPvFuture version must contain only hexadecimal digits")
		}
	}

	for index := 0; index < len(address); index++ {
		character := address[index]
		if isURIASCIIAlphaNumeric(character) || strings.ContainsRune("-._~!$&'()*+,;=:", rune(character)) {
			continue
		}

		return fmt.Errorf("character %q is not allowed in an IPvFuture address", character)
	}

	return nil
}

func validateURIPortSuffix(suffix string, requireDigits bool) error {
	if suffix == "" {
		return nil
	}

	if suffix[0] != ':' {
		return errors.New("unexpected characters after host")
	}

	if requireDigits && len(suffix) == 1 {
		return errors.New("port must contain decimal digits")
	}

	for index := 1; index < len(suffix); index++ {
		if suffix[index] < '0' || suffix[index] > '9' {
			return errors.New("port must contain only digits")
		}
	}

	return nil
}

func validURIScheme(scheme string) bool {
	if scheme == "" || !isURIASCIIAlpha(scheme[0]) {
		return false
	}

	for index := 1; index < len(scheme); index++ {
		character := scheme[index]
		if isURIASCIIAlphaNumeric(character) || strings.ContainsRune("+-.", rune(character)) {
			continue
		}

		return false
	}

	return true
}

func validateURIFragment(fragment string) error {
	return validateURIComponent(fragment, "-._~!$&'()*+,;=:@/?")
}

func validateURIComponent(value, allowedPunctuation string) error {
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '%' {
			if index+2 >= len(value) || !isHexDigit(value[index+1]) || !isHexDigit(value[index+2]) {
				return errors.New("invalid percent encoding")
			}

			index += 2

			continue
		}

		if isURIASCIIAlphaNumeric(character) || strings.ContainsRune(allowedPunctuation, rune(character)) {
			continue
		}

		return fmt.Errorf("character %q must be percent-encoded", character)
	}

	return nil
}

func isURIASCIIAlphaNumeric(character byte) bool {
	return isURIASCIIAlpha(character) || character >= '0' && character <= '9'
}

func isURIASCIIAlpha(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

func isHexDigit(character byte) bool {
	return character >= '0' && character <= '9' || character >= 'a' && character <= 'f' ||
		character >= 'A' && character <= 'F'
}
