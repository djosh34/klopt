//nolint:cyclop,gocognit,gocyclo,godoclint,maintidx,mnd,nestif // Accounting mirrors admitted keywords.
package program

import (
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/validation"
)

const (
	compileNodeBytes = 64
	compileFactBytes = 128
)

type compileMeasure struct {
	nodes uint64
	facts uint64
	bytes uint64
	seen  map[*validation.Validation]struct{}
	limit CompileLimits
}

func checkCompileLimits(roots []*validation.Validation, limits CompileLimits) error {
	measure := compileMeasure{seen: make(map[*validation.Validation]struct{}), limit: limits}

	rootBytes, ok := checkedMul(uint64(len(roots)), 4)
	if !ok {
		return &ResourceError{
			Resource: "compile bytes", Limit: limits.MaxProgramBytes, Observed: ^uint64(0),
		}
	}

	if err := measure.addBytes(rootBytes); err != nil {
		return err
	}

	for _, root := range roots {
		if root == nil {
			continue
		}

		if err := measure.validation(root); err != nil {
			return err
		}
	}

	return nil
}

//nolint:cyclop // Counting one admitted schema mirrors graph lowering exactly.
func (measure *compileMeasure) validation(source *validation.Validation) error {
	if _, exists := measure.seen[source]; exists {
		return nil
	}

	if err := measure.addNode(false, 0); err != nil {
		return err
	}

	measure.seen[source] = struct{}{}

	addFact := func(content uint64) error {
		return measure.addNode(true, content)
	}
	addNumberFact := func(number jsonvalue.Number) error {
		content, err := exactValueBytes(jsonvalue.Value{
			Kind: jsonvalue.KindNumber, Number: number,
		})
		if err != nil {
			return err
		}

		return addFact(content)
	}

	if source.KindValidation.Type != "" {
		if err := addFact(uint64(len(source.KindValidation.Type))); err != nil {
			return err
		}
	}

	if len(source.EnumValidation.ExactValues) != 0 {
		content := uint64(0)

		for _, value := range source.EnumValidation.ExactValues {
			size, err := exactValueBytes(value)
			if err != nil {
				return err
			}

			var ok bool

			content, ok = checkedAdd(content, size)
			if !ok {
				return &ResourceError{
					Resource: "compile bytes", Limit: measure.limit.MaxProgramBytes,
					Observed: ^uint64(0),
				}
			}
		}

		if err := addFact(content); err != nil {
			return err
		}
	}

	if source.NumberValidation.Minimum != nil {
		if err := addNumberFact(source.NumberValidation.Minimum.ExactValue); err != nil {
			return err
		}
	}

	if source.NumberValidation.Maximum != nil {
		if err := addNumberFact(source.NumberValidation.Maximum.ExactValue); err != nil {
			return err
		}
	}

	if source.NumberValidation.ExactMultipleOf != nil {
		if err := addNumberFact(*source.NumberValidation.ExactMultipleOf); err != nil {
			return err
		}
	}

	if source.NumberValidation.Format != "" {
		if err := addFact(uint64(len(source.NumberValidation.Format))); err != nil {
			return err
		}
	}

	if source.StringValidation.CompiledPattern != nil {
		if err := addFact(uint64(len(source.StringValidation.Pattern))); err != nil {
			return err
		}
	}

	if source.StringValidation.CompiledFormat != nil {
		if err := addFact(uint64(len(source.StringValidation.Format))); err != nil {
			return err
		}
	}

	for _, present := range []bool{
		source.StringValidation.MinLength != nil,
		source.StringValidation.MaxLength != nil,
		source.ArrayValidation.MinItems != nil,
		source.ArrayValidation.MaxItems != nil,
		source.ObjectValidation.MinProperties != nil,
		source.ObjectValidation.MaxProperties != nil,
	} {
		if present {
			if err := addFact(0); err != nil {
				return err
			}
		}
	}

	if source.ArrayValidation.Items != nil {
		if err := addFact(0); err != nil {
			return err
		}

		if err := measure.validation(source.ArrayValidation.Items); err != nil {
			return err
		}
	}

	for _, name := range source.ObjectValidation.Required {
		if err := addFact(uint64(len(name))); err != nil {
			return err
		}
	}

	for _, property := range source.ObjectValidation.Properties {
		if err := addFact(uint64(len(property.Name))); err != nil {
			return err
		}

		if err := measure.validation(property.Validation); err != nil {
			return err
		}
	}

	if !source.ObjectValidation.AdditionalPropertiesAllowed ||
		source.ObjectValidation.AdditionalPropertiesValidation != nil {
		content := uint64(0)

		for _, property := range source.ObjectValidation.Properties {
			var ok bool

			content, ok = checkedAdd(content, uint64(len(property.Name)))
			if !ok {
				return &ResourceError{
					Resource: "compile bytes", Limit: measure.limit.MaxProgramBytes,
					Observed: ^uint64(0),
				}
			}
		}

		if err := addFact(content); err != nil {
			return err
		}

		if source.ObjectValidation.AdditionalPropertiesValidation != nil {
			if err := measure.validation(source.ObjectValidation.AdditionalPropertiesValidation); err != nil {
				return err
			}
		}
	}

	for _, child := range source.AllOfValidations {
		if err := measure.validation(child); err != nil {
			return err
		}
	}

	if len(source.AnyOfValidations) != 0 {
		childBytes, ok := checkedMul(uint64(len(source.AnyOfValidations)), 4)
		if !ok {
			return &ResourceError{
				Resource: "compile bytes", Limit: measure.limit.MaxProgramBytes,
				Observed: ^uint64(0),
			}
		}

		if err := measure.addNode(false, childBytes); err != nil {
			return err
		}

		for _, child := range source.AnyOfValidations {
			if err := measure.validation(child); err != nil {
				return err
			}
		}
	}

	allOfCount, ok := checkedAdd(uint64(len(source.AllOfValidations)), 1)
	if !ok {
		return &ResourceError{
			Resource: "compile bytes", Limit: measure.limit.MaxProgramBytes,
			Observed: ^uint64(0),
		}
	}

	allOfBytes, ok := checkedMul(allOfCount, 4)
	if !ok {
		return &ResourceError{
			Resource: "compile bytes", Limit: measure.limit.MaxProgramBytes,
			Observed: ^uint64(0),
		}
	}

	return measure.addBytes(allOfBytes)
}

func (measure *compileMeasure) addNode(fact bool, content uint64) error {
	nodes, ok := checkedAdd(measure.nodes, 1)
	if !ok {
		return &ResourceError{Resource: "compile nodes", Limit: ^uint64(0), Observed: ^uint64(0)}
	}

	measure.nodes = nodes
	if measure.nodes > measure.limit.MaxNodes {
		return &ResourceError{
			Resource: "compile nodes", Limit: measure.limit.MaxNodes, Observed: measure.nodes,
		}
	}

	base := uint64(compileNodeBytes)

	if fact {
		facts, factsOK := checkedAdd(measure.facts, 1)
		if !factsOK {
			return &ResourceError{Resource: "compile facts", Limit: ^uint64(0), Observed: ^uint64(0)}
		}

		measure.facts = facts
		if measure.facts > measure.limit.MaxFacts {
			return &ResourceError{
				Resource: "compile facts", Limit: measure.limit.MaxFacts, Observed: measure.facts,
			}
		}

		base = compileFactBytes
	}

	bytes, ok := checkedAdd(base, content)
	if !ok {
		return &ResourceError{Resource: "compile bytes", Limit: ^uint64(0), Observed: ^uint64(0)}
	}

	return measure.addBytes(bytes)
}

func (measure *compileMeasure) addBytes(amount uint64) error {
	observed, ok := checkedAdd(measure.bytes, amount)
	if !ok {
		return &ResourceError{Resource: "compile bytes", Limit: ^uint64(0), Observed: ^uint64(0)}
	}

	measure.bytes = observed
	if measure.bytes > measure.limit.MaxProgramBytes {
		return &ResourceError{
			Resource: "compile bytes", Limit: measure.limit.MaxProgramBytes, Observed: measure.bytes,
		}
	}

	return nil
}

func exactValueBytes(value jsonvalue.Value) (uint64, error) {
	size := uint64(16)

	switch value.Kind {
	case jsonvalue.KindNumber:
		var err error

		size, err = checkedValueBytes(size, uint64(len(value.Number.Lexeme)))
		if err != nil || value.Number.Rational == nil {
			return size, err
		}

		size, err = checkedValueBytes(size, uint64(len(value.Number.Rational.Num().Bytes())))
		if err != nil {
			return 0, err
		}

		return checkedValueBytes(size, uint64(len(value.Number.Rational.Denom().Bytes())))
	case jsonvalue.KindString:
		return checkedValueBytes(size, uint64(len(value.String)))
	case jsonvalue.KindArray:
		for _, item := range value.Array {
			child, err := exactValueBytes(item)
			if err != nil {
				return 0, err
			}

			var ok bool

			size, ok = checkedAdd(size, child)
			if !ok {
				return 0, &ResourceError{
					Resource: "compile bytes", Limit: ^uint64(0), Observed: ^uint64(0),
				}
			}
		}
	case jsonvalue.KindObject:
		for _, member := range value.Object {
			child, err := exactValueBytes(member.Value)
			if err != nil {
				return 0, err
			}

			withName, ok := checkedAdd(uint64(len(member.Name)), child)
			if !ok {
				return 0, &ResourceError{Resource: "compile bytes", Limit: ^uint64(0), Observed: ^uint64(0)}
			}

			size, ok = checkedAdd(size, withName)
			if !ok {
				return 0, &ResourceError{
					Resource: "compile bytes", Limit: ^uint64(0), Observed: ^uint64(0),
				}
			}
		}
	}

	if size == ^uint64(0) {
		return 0, &ResourceError{Resource: "compile bytes", Limit: ^uint64(0), Observed: ^uint64(0)}
	}

	return size, nil
}

func checkedValueBytes(left uint64, right uint64) (uint64, error) {
	result, ok := checkedAdd(left, right)
	if !ok {
		return 0, &ResourceError{Resource: "compile bytes", Limit: ^uint64(0), Observed: ^uint64(0)}
	}

	return result, nil
}
