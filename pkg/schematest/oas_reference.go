//nolint:godoclint // Private RFC 6901 traversal keeps each container and escape failure explicit.
package schematest

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

func resolvePathItemReference(document, value *jsonValue, pointer string) (*jsonValue, string, error) {
	visited := make(map[string]bool)

	for {
		object, err := requireJSONObject(value, pointer)
		if err != nil {
			return nil, "", err
		}

		reference, referenced := object["$ref"]
		if !referenced {
			return value, pointer, nil
		}

		if reference.kind != jsonString {
			return nil, "", fmt.Errorf("%s/$ref: must be a string", pointer)
		}

		for _, name := range sortedObjectNames(object) {
			if name != "$ref" && !strings.HasPrefix(name, "x-") {
				return nil, "", fmt.Errorf(
					"%s/$ref: Path Item Object fields beside $ref have undefined OAS 3.0 behavior",
					pointer,
				)
			}
		}

		resolved, targetPointer, err := resolveLocalReference(document, reference.text, pointer+"/$ref")
		if err != nil {
			return nil, "", err
		}

		if visited[targetPointer] {
			return nil, "", fmt.Errorf(
				"%s/$ref: recursive path item reference reaching %s is outside the schematest profile",
				pointer,
				targetPointer,
			)
		}

		visited[targetPointer] = true
		value = resolved
		pointer = targetPointer
	}
}

func resolveReferenceChain(document, value *jsonValue, pointer, objectName string) (*jsonValue, string, error) {
	visited := make(map[string]bool)

	for {
		object, err := requireJSONObject(value, pointer)
		if err != nil {
			return nil, "", err
		}

		reference, referenced := object["$ref"]
		if !referenced {
			return value, pointer, nil
		}

		if reference.kind != jsonString {
			return nil, "", fmt.Errorf("%s/$ref: must be a string", pointer)
		}

		resolved, targetPointer, err := resolveLocalReference(document, reference.text, pointer+"/$ref")
		if err != nil {
			return nil, "", err
		}

		if visited[targetPointer] {
			return nil, "", fmt.Errorf(
				"%s/$ref: recursive %s reference reaching %s is outside the schematest profile",
				pointer,
				objectName,
				targetPointer,
			)
		}

		visited[targetPointer] = true
		value = resolved
		pointer = targetPointer
	}
}

func resolveLocalReference(document *jsonValue, reference, authoredPointer string) (*jsonValue, string, error) {
	fragment, err := parseLocalReferenceFragment(reference, authoredPointer)
	if err != nil {
		return nil, "", err
	}

	if fragment == "" {
		return document, "#", nil
	}

	current := document
	canonical := "#"

	for _, encodedToken := range strings.Split(fragment[1:], "/") {
		token, unescapeErr := unescapePointerToken(encodedToken)
		if unescapeErr != nil {
			return nil, "", fmt.Errorf("%s: malformed JSON Pointer token %q: %w", authoredPointer, encodedToken, unescapeErr)
		}

		canonical += "/" + escapePointerToken(token)

		switch current.kind {
		case jsonObject:
			next, exists := current.object[token]
			if !exists {
				return nil, "", fmt.Errorf("%s: local reference target %s does not exist", authoredPointer, canonical)
			}

			current = next
		case jsonArray:
			index, indexErr := pointerArrayIndex(token, len(current.array))
			if indexErr != nil {
				return nil, "", fmt.Errorf("%s: local reference target %s: %w", authoredPointer, canonical, indexErr)
			}

			current = current.array[index]
		default:
			return nil, "", fmt.Errorf("%s: local reference traverses through a non-container at %s", authoredPointer, canonical)
		}
	}

	return current, canonical, nil
}

func parseLocalReferenceFragment(reference, authoredPointer string) (string, error) {
	if !strings.HasPrefix(reference, "#") {
		return "", fmt.Errorf("%s: external reference %q is outside the schematest profile", authoredPointer, reference)
	}

	encodedFragment := strings.TrimPrefix(reference, "#")
	if err := validateURIFragment(encodedFragment); err != nil {
		return "", fmt.Errorf("%s: malformed URI-reference: %w", authoredPointer, err)
	}

	fragment, err := url.PathUnescape(encodedFragment)
	if err != nil {
		return "", fmt.Errorf("%s: malformed local reference: %w", authoredPointer, err)
	}

	if !utf8.ValidString(fragment) {
		return "", fmt.Errorf("%s: local reference fragment must be valid UTF-8", authoredPointer)
	}

	if fragment != "" && !strings.HasPrefix(fragment, "/") {
		return "", fmt.Errorf("%s: local reference fragment must be a JSON Pointer", authoredPointer)
	}

	return fragment, nil
}

func validateURIFragment(fragment string) error {
	for index := 0; index < len(fragment); index++ {
		character := fragment[index]
		if character == '%' {
			if index+2 >= len(fragment) || !isHexDigit(fragment[index+1]) || !isHexDigit(fragment[index+2]) {
				return errors.New("invalid percent encoding")
			}

			index += 2

			continue
		}

		if !isURIFragmentCharacter(character) {
			return fmt.Errorf("character %q must be percent-encoded", character)
		}
	}

	return nil
}

func isURIFragmentCharacter(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || strings.ContainsRune("-._~!$&'()*+,;=:@/?", rune(character))
}

func isHexDigit(character byte) bool {
	return character >= '0' && character <= '9' || character >= 'a' && character <= 'f' ||
		character >= 'A' && character <= 'F'
}

func unescapePointerToken(token string) (string, error) {
	decoded := make([]byte, 0, len(token))

	for index := 0; index < len(token); index++ {
		if token[index] != '~' {
			decoded = append(decoded, token[index])

			continue
		}

		if index+1 == len(token) {
			return "", errors.New("trailing '~'")
		}

		index++
		switch token[index] {
		case '0':
			decoded = append(decoded, '~')
		case '1':
			decoded = append(decoded, '/')
		default:
			return "", fmt.Errorf("unknown escape ~%c", token[index])
		}
	}

	return string(decoded), nil
}

func pointerArrayIndex(token string, length int) (int, error) {
	if token == "" || token == "-" || (len(token) > 1 && token[0] == '0') {
		return 0, fmt.Errorf("invalid array index %q", token)
	}

	index, err := strconv.ParseUint(token, 10, 64)
	if err != nil || index >= uint64(length) {
		return 0, fmt.Errorf("array index %q is out of range", token)
	}

	return int(index), nil
}
