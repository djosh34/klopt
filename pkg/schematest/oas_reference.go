//nolint:cyclop,godoclint // Private RFC 6901 traversal keeps each container and escape failure explicit.
package schematest

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

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
	if !strings.HasPrefix(reference, "#") {
		return nil, "", fmt.Errorf("%s: external reference %q is outside the schematest profile", authoredPointer, reference)
	}

	fragment, err := url.PathUnescape(strings.TrimPrefix(reference, "#"))
	if err != nil {
		return nil, "", fmt.Errorf("%s: malformed local reference: %w", authoredPointer, err)
	}

	if !utf8.ValidString(fragment) {
		return nil, "", fmt.Errorf("%s: local reference fragment must be valid UTF-8", authoredPointer)
	}

	if fragment == "" {
		return document, "#", nil
	}

	if !strings.HasPrefix(fragment, "/") {
		return nil, "", fmt.Errorf("%s: local reference fragment must be a JSON Pointer", authoredPointer)
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
