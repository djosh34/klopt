//nolint:godoclint // Package-private tests document the typed builder contract.
package testgenerator

import (
	"testing"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Tests exercise the shared string walk through builders.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

func TestBuildNullBooleanAndEnum(t *testing.T) {
	t.Parallel()

	nullResult := buildValue([]demand{newDemand(newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindNull: true},
	}), true)}, newTapeCursor(nil))
	require.Equal(t, buildComplete, nullResult.state)
	require.Equal(t, jsonvalue.Null(), nullResult.value)
	require.NoError(t, nullResult.err)

	falseResult := buildValue([]demand{newDemand(newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindBoolean: true},
	}), true)}, newTapeCursor(make([]byte, tapeWordBytes)))
	require.Equal(t, buildComplete, falseResult.state)
	require.Equal(t, jsonvalue.Bool(false), falseResult.value)

	trueTape := make([]byte, tapeWordBytes)
	trueTape[0] = 1
	trueResult := buildValue([]demand{newDemand(newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindBoolean: true},
	}), true)}, newTapeCursor(trueTape))
	require.Equal(t, buildComplete, trueResult.state)
	require.Equal(t, jsonvalue.Bool(true), trueResult.value)

	number, err := jsonvalue.ParseNumber("7")
	require.NoError(t, err)

	enumResult := buildValue([]demand{newDemand(newAtomExpression(atom{
		kind:   atomEnum,
		values: []jsonvalue.Value{{Kind: jsonvalue.KindNumber, Number: number}},
	}), true)}, newTapeCursor(nil))
	require.Equal(t, buildComplete, enumResult.state)
	require.Equal(t, jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number}, enumResult.value)

	enumValues := []jsonvalue.Value{
		jsonvalue.String("text"),
		jsonvalue.Array([]jsonvalue.Value{jsonvalue.Bool(true)}),
		mustObjectValue(t, []jsonvalue.Member{{Name: "name", Value: jsonvalue.String("value")}}),
	}
	for _, expected := range enumValues {
		result := buildValue([]demand{newDemand(newAtomExpression(atom{
			kind:   atomEnum,
			values: []jsonvalue.Value{expected},
		}), true)}, newTapeCursor(nil))
		require.Equal(t, buildComplete, result.state)
		require.True(t, expected.Equal(result.value))
	}
}

func TestBuildPositiveAndNegativeTypedDemands(t *testing.T) {
	t.Parallel()

	positiveString := buildValue([]demand{
		newDemand(newAtomExpression(atom{
			kind:    atomKinds,
			allowed: [jsonKindCount]bool{kindString: true},
		}), true),
		newDemand(newAtomExpression(atom{
			kind:  atomStringMinLength,
			count: mustNumber(t, "1"),
		}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, positiveString.state)
	require.Equal(t, 1, len([]rune(positiveString.value.String)))

	negativeString := buildValue([]demand{
		newDemand(newAtomExpression(atom{
			kind:  atomStringMinLength,
			count: mustNumber(t, "1"),
		}), false),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, negativeString.state)
	require.Equal(t, jsonvalue.String(""), negativeString.value)

	nonApplicable := buildValue([]demand{
		newDemand(newAtomExpression(atom{
			kind:  atomStringMinLength,
			count: mustNumber(t, "1"),
		}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, nonApplicable.state)
	require.Equal(t, jsonvalue.Null(), nonApplicable.value)

	negativeInteger := buildValue([]demand{newDemand(newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindNumber: true},
		integer: true,
	}), false)}, newTapeCursor(nil))
	require.Equal(t, buildComplete, negativeInteger.state)
	require.Equal(t, "0.5", negativeInteger.value.Number.Lexeme)

	negativeEnum := buildValue([]demand{
		newDemand(newAtomExpression(atom{kind: atomKinds, allowed: [jsonKindCount]bool{kindNumber: true}}), true),
		newDemand(newAtomExpression(atom{
			kind:   atomEnum,
			values: []jsonvalue.Value{{Kind: jsonvalue.KindNumber, Number: mustNumber(t, "0")}},
		}), false),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, negativeEnum.state)
	require.Equal(t, "1", negativeEnum.value.Number.Lexeme)
}

func TestBuildExactNumberDomains(t *testing.T) {
	t.Parallel()

	positive := []demand{
		newDemand(newAtomExpression(atom{
			kind:    atomKinds,
			allowed: [jsonKindCount]bool{kindNumber: true},
		}), true),
		newDemand(newAtomExpression(atom{kind: atomNumberMinimum, number: mustNumber(t, "1")}), true),
		newDemand(newAtomExpression(atom{kind: atomNumberMaximum, number: mustNumber(t, "5")}), true),
		newDemand(newAtomExpression(atom{kind: atomNumberMultipleOf, number: mustNumber(t, "2")}), true),
	}
	result := buildValue(positive, newTapeCursor(nil))
	require.Equal(t, buildComplete, result.state)
	require.Equal(t, "2", result.value.Number.Lexeme)

	exclusive := buildValue([]demand{
		newDemand(newAtomExpression(atom{
			kind:    atomKinds,
			allowed: [jsonKindCount]bool{kindNumber: true},
		}), true),
		newDemand(newAtomExpression(atom{
			kind:      atomNumberMinimum,
			number:    mustNumber(t, "1"),
			exclusive: true,
		}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, exclusive.state)
	require.Equal(t, "2", exclusive.value.Number.Lexeme)

	for _, format := range []string{"int32", "int64", "float", "double"} {
		formatted := buildValue([]demand{
			newDemand(newAtomExpression(atom{
				kind:    atomKinds,
				allowed: [jsonKindCount]bool{kindNumber: true},
			}), true),
			newDemand(newAtomExpression(atom{kind: atomNumberFormat, text: format}), true),
		}, newTapeCursor(nil))
		require.Equal(t, buildComplete, formatted.state, format)
	}

	contradictory := buildValue([]demand{
		newDemand(newAtomExpression(atom{
			kind:    atomKinds,
			allowed: [jsonKindCount]bool{kindNumber: true},
		}), true),
		newDemand(newAtomExpression(atom{kind: atomNumberMinimum, number: mustNumber(t, "5")}), true),
		newDemand(newAtomExpression(atom{kind: atomNumberMaximum, number: mustNumber(t, "1")}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildMiss, contradictory.state)
}

func TestBuildStringMinimumBeforeZeroStop(t *testing.T) {
	t.Parallel()

	withMinimum := buildValue([]demand{
		newDemand(newAtomExpression(atom{
			kind:    atomKinds,
			allowed: [jsonKindCount]bool{kindString: true},
		}), true),
		newDemand(newAtomExpression(atom{kind: atomStringMinLength, count: mustNumber(t, "2")}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, withMinimum.state)
	require.Equal(t, 2, len([]rune(withMinimum.value.String)))

	empty := buildValue([]demand{newDemand(newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindString: true},
	}), true)}, newTapeCursor(nil))
	require.Equal(t, buildComplete, empty.state)
	require.Equal(t, jsonvalue.String(""), empty.value)
}

func TestBuildStringUsesOneSharedRuneStream(t *testing.T) {
	t.Parallel()

	positive, err := stringlanguage.Pattern(".")
	require.NoError(t, err)
	negative, err := stringlanguage.Pattern("z")
	require.NoError(t, err)

	result := buildString([]demand{
		newDemand(newAtomExpression(atom{kind: atomStringLanguage, language: positive}), true),
		newDemand(newAtomExpression(atom{kind: atomStringLanguage, language: negative}), false),
		newDemand(newAtomExpression(atom{kind: atomStringMinLength, count: mustNumber(t, "1")}), true),
		newDemand(newAtomExpression(atom{kind: atomStringMaxLength, count: mustNumber(t, "1")}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, result.state)
	require.Len(t, []rune(result.value.String), 1)
}

func TestBuildArrayMinimumBeforeZeroStop(t *testing.T) {
	t.Parallel()

	item := newAllExpression([]*expression{newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindNumber: true},
		integer: true,
	})})
	result := buildValue([]demand{
		newDemand(newAtomExpression(atom{
			kind:    atomKinds,
			allowed: [jsonKindCount]bool{kindArray: true},
		}), true),
		newDemand(newAtomExpression(atom{kind: atomArrayMinItems, count: mustNumber(t, "2")}), true),
		newDemand(newAtomExpression(atom{kind: atomArrayItems, child: item}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, result.state)
	require.Len(t, result.value.Array, 2)
}

func TestBuildObjectRequiredAndMinimumBeforeZeroStop(t *testing.T) {
	t.Parallel()

	stringChild := newAllExpression([]*expression{newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindString: true},
	})})
	booleanChild := newAllExpression([]*expression{newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindBoolean: true},
	})})
	result := buildValue([]demand{
		newDemand(newAtomExpression(atom{
			kind:    atomKinds,
			allowed: [jsonKindCount]bool{kindObject: true},
		}), true),
		newDemand(newAtomExpression(atom{kind: atomObjectRequired, names: []string{"required"}}), true),
		newDemand(newAtomExpression(atom{kind: atomObjectMinProperties, count: mustNumber(t, "2")}), true),
		newDemand(newAtomExpression(atom{kind: atomObjectProperty, name: "optional", child: booleanChild}), true),
		newDemand(newAtomExpression(atom{kind: atomObjectProperty, name: "required", child: stringChild}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, result.state)
	require.Len(t, result.value.Object, 2)
	require.True(t, hasMember(result.value.Object, "required"))
	require.True(t, hasMember(result.value.Object, "optional"))
}

func TestBuildNegativeRequiredDemandOmitsOneName(t *testing.T) {
	t.Parallel()

	child := newAllExpression([]*expression{newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindString: true},
	})})
	result := buildValue([]demand{
		newDemand(newAtomExpression(atom{kind: atomKinds, allowed: [jsonKindCount]bool{kindObject: true}}), true),
		newDemand(newAtomExpression(atom{kind: atomObjectRequired, names: []string{"name"}}), false),
		newDemand(newAtomExpression(atom{kind: atomObjectProperty, name: "name", child: child}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildComplete, result.state)
	require.False(t, hasMember(result.value.Object, "name"))
}

func TestBuildMissDoesNotRetryAnotherKindBranchOrItem(t *testing.T) {
	t.Parallel()

	badChild := newAllExpression([]*expression{
		newAtomExpression(atom{kind: atomKinds, allowed: [jsonKindCount]bool{kindString: true}}),
		newAtomExpression(atom{
			kind:   atomEnum,
			values: []jsonvalue.Value{{Kind: jsonvalue.KindNumber, Number: mustNumber(t, "1")}},
		}),
	})
	result := buildValue([]demand{
		newDemand(newAtomExpression(atom{kind: atomKinds, allowed: [jsonKindCount]bool{kindArray: true}}), true),
		newDemand(newAtomExpression(atom{kind: atomArrayMinItems, count: mustNumber(t, "1")}), true),
		newDemand(newAtomExpression(atom{kind: atomArrayItems, child: badChild}), true),
	}, newTapeCursor(nil))
	require.Equal(t, buildMiss, result.state)
}

func mustNumber(t *testing.T, lexeme string) jsonvalue.Number {
	t.Helper()

	number, err := jsonvalue.ParseNumber(lexeme)
	require.NoError(t, err)

	return number
}

func mustObjectValue(t *testing.T, members []jsonvalue.Member) jsonvalue.Value {
	t.Helper()

	value, err := jsonvalue.Object(members)
	require.NoError(t, err)

	return value
}
