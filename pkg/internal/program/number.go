//nolint:cyclop,gocognit,gocyclo,godoclint,maintidx,mnd,nestif // Exact numeric sampling stays private.
package program

import (
	"fmt"
	"math/big"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func (program *Program) sampleNumber(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
) (jsonvalue.Number, bool, error) {
	interval := numberInterval{}

	var step *big.Rat

	requireInteger := false
	forbidInteger := false
	hasNegativeFormat := false
	minimumScale := uint64(0)

	for _, current := range goals {
		item := program.nodes[current.node].atom

		switch item.kind {
		case atomKinds:
			if !item.integer {
				continue
			}

			if current.want {
				requireInteger = true
			} else {
				forbidInteger = true
			}
		case atomNumberMinimum:
			value, err := materializedRat(item.number, work)
			if err != nil {
				return jsonvalue.Number{}, false, err
			}

			if current.want {
				interval.addLower(value, item.exclusive)
			} else {
				interval.addUpper(value, !item.exclusive)
			}
		case atomNumberMaximum:
			value, err := materializedRat(item.number, work)
			if err != nil {
				return jsonvalue.Number{}, false, err
			}

			if current.want {
				interval.addUpper(value, item.exclusive)
			} else {
				interval.addLower(value, !item.exclusive)
			}
		case atomNumberMultipleOf:
			value, err := materializedRat(item.number, work)
			if err != nil {
				return jsonvalue.Number{}, false, err
			}

			value.Abs(value)

			if current.want {
				step = rationalLCM(step, value)
			} else {
				scale, scaleErr := finiteScale(value)
				if scaleErr != nil {
					return jsonvalue.Number{}, false, scaleErr
				}

				minimumScale = max(minimumScale, uint64(scale+1))
			}
		case atomNumberFormat:
			if current.want {
				switch item.text {
				case "int32":
					requireInteger = true

					interval.addLower(new(big.Rat).SetInt64(-1<<31), false)
					interval.addUpper(new(big.Rat).SetInt64(1<<31-1), false)
				case "int64":
					requireInteger = true

					interval.addLower(new(big.Rat).SetInt64(-1<<63), false)
					interval.addUpper(new(big.Rat).SetInt64(1<<63-1), false)
				}
			} else {
				hasNegativeFormat = true
			}
		}
	}

	if requireInteger && forbidInteger {
		return jsonvalue.Number{}, false, nil
	}

	if forbidInteger {
		minimumScale = max(minimumScale, 1)
	}

	if requireInteger {
		step = rationalLCM(step, new(big.Rat).SetInt64(1))
	}

	if interval.empty() {
		return jsonvalue.Number{}, false, nil
	}

	if hasNegativeFormat {
		candidate, possible, err := program.sampleFormatFault(goals, excluded, reader)
		if err != nil || possible {
			return candidate, possible, err
		}

		return jsonvalue.Number{}, false, nil
	}

	var (
		unit   *big.Rat
		rank   *big.Int
		domain integerDomain
	)

	if step != nil {
		unit = step
		domain = interval.integerDomain(unit)

		var err error

		rank, err = readNatural(reader, work)
		if err != nil {
			return jsonvalue.Number{}, false, err
		}
	} else {
		scaleRank, err := readNatural(reader, work)
		if err != nil {
			return jsonvalue.Number{}, false, err
		}

		if !scaleRank.IsUint64() {
			return jsonvalue.Number{}, false, &ResourceError{
				Resource: "decimal scale", Limit: work.limits.MaxSolverBytes, Observed: ^uint64(0),
			}
		}

		scale, ok := checkedAdd(scaleRank.Uint64(), minimumScale)
		if !ok {
			return jsonvalue.Number{}, false, &ResourceError{
				Resource: "decimal scale", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		if scale > work.limits.MaxSolverBytes || scale > uint64(int(^uint(0)>>1)) {
			return jsonvalue.Number{}, false, &ResourceError{
				Resource: "decimal scale", Limit: work.limits.MaxSolverBytes, Observed: scale,
			}
		}

		denominator := new(big.Int).Exp(big.NewInt(10), new(big.Int).SetUint64(scale), nil)
		unit = new(big.Rat).SetFrac(big.NewInt(1), denominator)
		domain = interval.integerDomain(unit)

		rank, err = readNatural(reader, work)
		if err != nil {
			return jsonvalue.Number{}, false, err
		}
	}

	if domain.empty() {
		return jsonvalue.Number{}, false, nil
	}

	attempts := new(big.Int)

	for {
		if err := work.solver(uint64(len(rank.Bytes())) + 32); err != nil {
			return jsonvalue.Number{}, false, err
		}

		coefficient := domain.at(rank)
		candidate := new(big.Rat).Mul(new(big.Rat).SetInt(coefficient), unit)

		lexeme, err := finiteDecimal(candidate, work)
		if err != nil {
			return jsonvalue.Number{}, false, err
		}

		number, err := jsonvalue.ParseNumber(lexeme)
		if err != nil {
			return jsonvalue.Number{}, false, fmt.Errorf("parse generated number %q: %w", lexeme, err)
		}

		matches, matchErr := program.valueAllowed(goals, excluded, jsonvalue.Value{
			Kind: jsonvalue.KindNumber, Number: number,
		})
		if matchErr != nil {
			return jsonvalue.Number{}, false, matchErr
		}

		if matches {
			return number, true, nil
		}

		rank.Add(rank, big.NewInt(1))
		attempts.Add(attempts, big.NewInt(1))

		if count := domain.count(); count != nil && attempts.Cmp(count) >= 0 {
			return jsonvalue.Number{}, false, nil
		}
	}
}
