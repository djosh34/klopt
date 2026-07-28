//nolint:godoclint // Private integration helpers are intentionally concise.
package testgenerator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func compactJSON(body []byte) (string, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		return "", err
	}

	return compact.String(), nil
}

func requestBodySpec(schema string) []byte {
	lines := strings.Split(strings.Trim(schema, "\n"), "\n")

	indent := len(lines[0]) - len(strings.TrimLeft(lines[0], " "))
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			indent = min(indent, len(line)-len(strings.TrimLeft(line, " ")))
		}
	}

	for index := range lines {
		lines[index] = "              " + lines[index][min(indent, len(lines[index])):]
	}

	return fmt.Appendf(nil, `
openapi: 3.0.3
info:
  title: contract test
  version: 1.0.0
paths:
  /things:
    post:
      operationId: checkThing
      requestBody:
        content:
          application/json:
            schema:
%s
      responses:
        '204':
          description: accepted
`, strings.Join(lines, "\n"))
}

// TestCheckJSONRequestBodiesSkipsQueryOnlyOperations verifies non-body operations need no generated suite.
func TestCheckJSONRequestBodiesSkipsQueryOnlyOperations(t *testing.T) {
	t.Parallel()

	CheckJSONRequestBodies(t, []byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - {name: limit, in: query, schema: {type: integer}}
`), func(operationID string, body []byte) error {
		t.Fatalf("validator called for query-only operation %q with %s", operationID, body)

		return nil
	}, validation.PatternOptions())
}

// TestCheckJSONRequestBodiesRunsCompiledPartitionsAsValidJSON verifies compiled partitions and operation routing.
func TestCheckJSONRequestBodiesRunsCompiledPartitionsAsValidJSON(t *testing.T) {
	t.Parallel()

	spec := requestBodySpec(`
      enum: [null, true, 1, "λ", [], {}]
`)

	var calls atomic.Int64

	t.Cleanup(func() {
		require.Greater(t, calls.Load(), int64(6))
	})

	CheckJSONRequestBodies(t, spec, func(operationID string, body []byte) error {
		require.Equal(t, "checkThing", operationID)
		require.True(t, json.Valid(body))

		calls.Add(1)

		compact, err := compactJSON(body)
		require.NoError(t, err)

		for _, accepted := range []string{`null`, `true`, `1`, `"λ"`, `[]`, `{}`} {
			if compact == accepted {
				return nil
			}
		}

		return errors.New("not an enum member")
	}, validation.PatternOptions())
}

// TestCheckJSONRequestBodiesRoutesEveryExactOperationID verifies one parse and document-wide dispatch.
func TestCheckJSONRequestBodiesRoutesEveryExactOperationID(t *testing.T) {
	t.Parallel()

	spec := []byte(`
openapi: 3.0.3
paths:
  /upper:
    post:
      operationId: Case/Sensitive
      requestBody:
        content:
          application/json:
            schema: {type: string}
  /lower:
    post:
      operationId: case
      requestBody:
        content:
          application/json:
            schema: {type: boolean}
`)

	seen := make(map[string]int)

	var mutex sync.Mutex

	t.Cleanup(func() {
		mutex.Lock()
		defer mutex.Unlock()

		require.Positive(t, seen["Case/Sensitive"])
		require.Positive(t, seen["case"])
		require.Len(t, seen, 2)
	})

	CheckJSONRequestBodies(t, spec, func(operationID string, body []byte) error {
		mutex.Lock()
		seen[operationID]++
		mutex.Unlock()

		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			return err
		}

		switch operationID {
		case "Case/Sensitive":
			if _, ok := value.(string); !ok {
				return errors.New("not a string")
			}
		case "case":
			if _, ok := value.(bool); !ok {
				return errors.New("not a boolean")
			}
		default:
			return fmt.Errorf("unexpected operationId %q", operationID)
		}

		return nil
	}, validation.PatternOptions())
}

// TestCheckJSONRequestBodiesConstructsPatternCasesThroughCallback verifies the normal public callback path.
func TestCheckJSONRequestBodiesConstructsPatternCasesThroughCallback(t *testing.T) {
	t.Parallel()

	spec := requestBodySpec(`
      type: string
      minLength: 2
      maxLength: 4
      allOf:
        - pattern: '^[A-Z]+$'
        - pattern: '^A'
`)

	var (
		accepted atomic.Int64
		rejected atomic.Int64
	)

	t.Cleanup(func() {
		require.Positive(t, accepted.Load())
		require.Positive(t, rejected.Load())
	})

	first := patternvalidator.MustParse(`^[A-Z]+$`)
	second := patternvalidator.MustParse(`^A`)

	CheckJSONRequestBodies(t, spec, func(operationID string, body []byte) error {
		require.Equal(t, "checkThing", operationID)

		var decoded any
		require.NoError(t, json.Unmarshal(body, &decoded))

		value, stringValue := decoded.(string)
		if !stringValue {
			rejected.Add(1)

			return errors.New("type rejected")
		}

		valid := utf8.RuneCountInString(value) >= 2 && utf8.RuneCountInString(value) <= 4 &&
			first.Validate(value) && second.Validate(value)
		if valid {
			accepted.Add(1)

			return nil
		}

		rejected.Add(1)

		return errors.New("pattern rejected")
	}, validation.PatternOptions())
}
