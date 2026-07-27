//nolint:godoclint // Internal constructor and helper tests deliberately use private names.
package validation

import (
	"testing"

	"github.com/go-json-experiment/json/jsontext"
	"github.com/stretchr/testify/require"
)

func TestPathDecoderCompilesOneRegexpPerSegment(t *testing.T) {
	t.Parallel()

	stringParameter := func(name string) pathParameter {
		return pathParameter{
			name: name, wire: pathWireSimplePrimitive,
			validation: &Validation{KindValidation: KindValidation{Type: "string"}},
			scalarType: "string",
		}
	}

	decoder, err := newPathDecoder(
		"segments",
		"//literal/{x}-{y}/",
		[]pathParameter{stringParameter("x"), stringParameter("y")},
	)
	require.NoError(t, err)
	require.Len(t, decoder.segments, 5)
	require.Equal(t, []int{0, 0, 0, 2, 0}, []int{
		decoder.segments[0].NumSubexp(),
		decoder.segments[1].NumSubexp(),
		decoder.segments[2].NumSubexp(),
		decoder.segments[3].NumSubexp(),
		decoder.segments[4].NumSubexp(),
	})
	require.Equal(t, `^((?:%[0-9A-Fa-f]{2}|[^%/])*?)-((?:%[0-9A-Fa-f]{2}|[^%/])*?)$`, decoder.segments[3].String())
	require.Equal(t, []string{"", "", ""}, decoder.segments[3].SubexpNames())
}

func TestSharedPathDecoderConstructorRejectsInvalidMetadata(t *testing.T) {
	t.Parallel()

	decoder, err := newPathDecoder("private", "/{p}", []pathParameter{{
		name: "p", wire: pathWireKind(255), validation: &Validation{},
	}})
	require.Nil(t, decoder)
	require.Error(t, err)
}

func TestPrivatePathHelpersReturnDefensiveErrors(t *testing.T) {
	t.Parallel()

	_, err := decodePathToken("%zz")
	require.Error(t, err)

	_, err = encodePathObject(
		[]pathParameter{{name: "p"}},
		[]jsontext.Value{jsontext.Value(`not JSON`)},
	)
	require.Error(t, err)
}
