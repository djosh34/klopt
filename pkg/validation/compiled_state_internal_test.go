//nolint:godoclint // White-box allocation counts cover generated graph restoration.
package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Per-process allocation counts must run without concurrent tests.
func TestGeneratedRestorationValidatesSharedGraphOnce(t *testing.T) {
	const (
		depth          = 64
		parameterCount = 16
	)

	shared := new(Validation)
	for range depth {
		shared = &Validation{AllOfValidations: []*Validation{shared}}
	}

	for _, test := range []struct {
		name    string
		restore func(*Validation) error
	}{
		{name: "query", restore: func(root *Validation) error {
			parameters := make([]QueryParameterDefinition, parameterCount)
			for index := range parameters {
				parameters[index] = QueryParameterDefinition{
					Name: fmt.Sprintf("q%d", index), Wire: uint8(wireJSONContent), Validation: root,
				}
			}

			_, err := NewQueryDecoderFromGenerated(QueryDecoderDefinition{
				OperationID: "query", Parameters: parameters,
			})

			return err
		}},
		{name: "path", restore: func(root *Validation) error {
			parameters := make([]PathParameterDefinition, parameterCount)
			segments := make([]string, parameterCount)

			for index := range parameters {
				name := fmt.Sprintf("p%d", index)
				parameters[index] = PathParameterDefinition{
					Name: name, Wire: uint8(pathWireJSONContent), Validation: root,
				}
				segments[index] = "{" + name + "}"
			}

			_, err := NewPathDecoderFromGenerated(PathDecoderDefinition{
				OperationID: "path", PathTemplate: "/" + strings.Join(segments, "/"), Parameters: parameters,
			})

			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			allocations := func(root *Validation) float64 {
				return testing.AllocsPerRun(3, func() {
					if err := test.restore(root); err != nil {
						panic(err)
					}
				})
			}

			shallow := allocations(new(Validation))
			deep := allocations(shared)
			require.Less(t, deep-shallow, float64(depth*2))
		})
	}
}
