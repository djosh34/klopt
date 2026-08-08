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

	command := exec.Command(os.Args[0], "-test.run=^TestDeepYAMLBuildFinishesWithoutProcessStackFailure$", "-test.count=1")

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

	command := exec.Command(os.Args[0], "-test.run=^TestDeepJSONBuildFinishesWithoutProcessStackFailure$", "-test.count=1")

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
