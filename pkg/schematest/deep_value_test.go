package schematest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestZeroStepBuildTraversesDeepJSONEnum exercises deep JSON through normal Build admission.
func TestZeroStepBuildTraversesDeepJSONEnum(t *testing.T) {
	t.Parallel()

	requireDeepZeroStepBuild(t, deepEnumJSONDocument(20_000))
}

// TestZeroStepBuildTraversesDeepYAMLEnum exercises deep YAML through normal Build admission.
func TestZeroStepBuildTraversesDeepYAMLEnum(t *testing.T) {
	t.Parallel()

	const depth = 2_000

	value := strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth)
	document := fmt.Sprintf(`openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              enum: [%s, %s]
`, value, value)

	requireDeepZeroStepBuild(t, []byte(document))
}

// TestJSONValueInternerDeduplicatesNestedSemanticValuesDeterministically checks iterative canonicalization.
func TestJSONValueInternerDeduplicatesNestedSemanticValuesDeterministically(t *testing.T) {
	t.Parallel()

	one, err := parseExactNumber("1")
	require.NoError(t, err)
	onePointZero, err := parseExactNumber("1.0")
	require.NoError(t, err)

	first := &jsonValue{kind: jsonObject, object: map[string]*jsonValue{
		"ä": {kind: jsonArray, array: []*jsonValue{{kind: jsonNumber, number: onePointZero}}},
		"z": {kind: jsonBoolean, boolean: true},
	}}
	second := &jsonValue{kind: jsonObject, object: map[string]*jsonValue{
		"z": {kind: jsonBoolean, boolean: true},
		"ä": {kind: jsonArray, array: []*jsonValue{{kind: jsonNumber, number: one}}},
	}}
	interner := jsonValueInterner{
		valueIDs: make(map[*jsonValue]int),
		shapeIDs: make(map[string]int),
		visiting: make(map[*jsonValue]bool),
	}

	firstID, err := interner.intern(first)
	require.NoError(t, err)
	secondID, err := interner.intern(second)
	require.NoError(t, err)
	require.Equal(t, firstID, secondID)
}

// TestJSONValueInternerRejectsCycle checks malformed private enum state.
func TestJSONValueInternerRejectsCycle(t *testing.T) {
	t.Parallel()

	cycle := &jsonValue{kind: jsonArray}
	cycle.array = []*jsonValue{cycle}
	interner := jsonValueInterner{
		valueIDs: make(map[*jsonValue]int),
		shapeIDs: make(map[string]int),
		visiting: make(map[*jsonValue]bool),
	}

	_, err := interner.intern(cycle)
	require.ErrorContains(t, err, "cycle")
}

// TestDeepYAMLBuildFinishesWithoutProcessStackFailure isolates a fatal-stack regression probe.
func TestDeepYAMLBuildFinishesWithoutProcessStackFailure(t *testing.T) {
	t.Parallel()

	if os.Getenv("SCHEMATEST_DEEP_YAML_PROBE") == "1" {
		const depth = 20_000

		value := strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth)
		document := fmt.Sprintf(`openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              default: %s
`, value)
		requireDeepZeroStepBuild(t, []byte(document))

		return
	}

	command := exec.CommandContext(
		t.Context(), os.Args[0], "-test.run=^TestDeepYAMLBuildFinishesWithoutProcessStackFailure$", "-test.count=1",
	)

	command.Env = append(os.Environ(), "SCHEMATEST_DEEP_YAML_PROBE=1")
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

// TestDeepJSONBuildFinishesWithoutProcessStackFailure isolates a fatal-stack regression probe.
func TestDeepJSONBuildFinishesWithoutProcessStackFailure(t *testing.T) {
	t.Parallel()

	if os.Getenv("SCHEMATEST_DEEP_JSON_PROBE") == "1" {
		requireDeepZeroStepBuild(t, deepEnumJSONDocument(250_000))

		return
	}

	command := exec.CommandContext(
		t.Context(), os.Args[0], "-test.run=^TestDeepJSONBuildFinishesWithoutProcessStackFailure$", "-test.count=1",
	)

	command.Env = append(os.Environ(), "SCHEMATEST_DEEP_JSON_PROBE=1")
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

// requireDeepZeroStepBuild requires complete admission without assignment or callback work.
func requireDeepZeroStepBuild(t *testing.T, document []byte) {
	t.Helper()

	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 0}, func(Case) error {
		return errors.New("zero-step Build invoked callback")
	})
	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Zero(t, report.Steps)
}

// deepEnumJSONDocument builds two semantically equal deeply nested enum members.
func deepEnumJSONDocument(depth int) []byte {
	value := strings.Repeat("[", depth) + "1.0" + strings.Repeat("]", depth)
	prefix := `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected",` +
		`"requestBody":{"content":{"application/json":{"schema":{"enum":[`
	suffix := `]}}}}}}}}`

	return []byte(prefix + value + "," + value + suffix)
}
