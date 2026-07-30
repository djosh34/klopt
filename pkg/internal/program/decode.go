//nolint:godoclint // Private execution mechanics stay behind Program.Decode.
package program

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/bits"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type constructionKind uint8

const (
	constructionString constructionKind = iota
	constructionArray
	constructionObject
)

type constructionFrame struct {
	kind  constructionKind
	count uint64
}

type returnKind uint8

const (
	returnNode returnKind = iota
	returnArraySequence
)

type returnFrame struct {
	kind      returnKind
	resume    NodeID
	child     NodeID
	remaining uint64
}

type execution struct {
	program       *Program
	limits        Limits
	reader        tapeReader
	steps         uint64
	output        []byte
	constructions []constructionFrame
	returns       []returnFrame
}

//nolint:cyclop,gocognit,gocyclo,maintidx // One interpreter keeps charges beside guarded work.
func (program *Program) decode(tape []byte, limits Limits) (jsonvalue.Value, error) {
	if program == nil || len(program.nodes) == 0 || uint64(program.root) >= uint64(len(program.nodes)) {
		return jsonvalue.Value{}, fmt.Errorf("decode invalid program")
	}

	run := execution{program: program, limits: limits, reader: tapeReader{tape: tape}}
	current := program.root

	for {
		if uint64(current) >= uint64(len(program.nodes)) {
			return jsonvalue.Value{}, fmt.Errorf("sealed continuation node %d is invalid", current)
		}

		node := program.nodes[current]
		if len(node.outgoing) == 0 {
			return jsonvalue.Value{}, fmt.Errorf("sealed node %d has no transitions", current)
		}

		transitionID, err := program.chooseTransition(node.outgoing, &run.reader)
		if err != nil {
			return jsonvalue.Value{}, err
		}

		if err := run.chargeStep(); err != nil {
			return jsonvalue.Value{}, err
		}

		item := program.transitions[transitionID]

		switch item.kind {
		case transitionScalar:
			if err := run.ensureString(); err != nil {
				return jsonvalue.Value{}, err
			}

			scalar := chooseScalar(item.ranges, &run.reader)

			encoded, marshalErr := json.Marshal(string(scalar))
			if marshalErr != nil {
				return jsonvalue.Value{}, fmt.Errorf("encode Unicode scalar: %w", marshalErr)
			}

			if err := run.appendOutput(encoded[1 : len(encoded)-1]); err != nil {
				return jsonvalue.Value{}, err
			}

			current = item.next
		case transitionStringStop:
			if err := run.ensureString(); err != nil {
				return jsonvalue.Value{}, err
			}

			if err := run.appendOutput([]byte{'"'}); err != nil {
				return jsonvalue.Value{}, err
			}

			run.constructions = run.constructions[:len(run.constructions)-1]

			next, complete, returnErr := run.completeValue()
			if returnErr != nil {
				return jsonvalue.Value{}, returnErr
			}

			if complete {
				return run.finish()
			}

			current = next
		case transitionBeginString:
			if err := run.begin(constructionString, '"'); err != nil {
				return jsonvalue.Value{}, err
			}

			current = item.next
		case transitionExactValue:
			if err := run.checkValueDepth(item.valueDepth); err != nil {
				return jsonvalue.Value{}, err
			}

			if err := run.appendOutput(item.valueJSON); err != nil {
				return jsonvalue.Value{}, err
			}

			next, complete, returnErr := run.completeValue()
			if returnErr != nil {
				return jsonvalue.Value{}, returnErr
			}

			if complete {
				return run.finish()
			}

			current = next
		case transitionInteger:
			encoded, integerErr := run.decodeInteger(item.minimum, item.maximum)
			if integerErr != nil {
				return jsonvalue.Value{}, integerErr
			}

			if err := run.checkValueDepth(1); err != nil {
				return jsonvalue.Value{}, err
			}

			if err := run.appendOutput(encoded); err != nil {
				return jsonvalue.Value{}, err
			}

			next, complete, returnErr := run.completeValue()
			if returnErr != nil {
				return jsonvalue.Value{}, returnErr
			}

			if complete {
				return run.finish()
			}

			current = next
		case transitionBeginArray:
			if err := run.begin(constructionArray, '['); err != nil {
				return jsonvalue.Value{}, err
			}

			current = item.next
		case transitionBeginObject:
			if err := run.begin(constructionObject, '{'); err != nil {
				return jsonvalue.Value{}, err
			}

			current = item.next
		case transitionArrayItem:
			if err := run.beginElement(constructionArray, nil); err != nil {
				return jsonvalue.Value{}, err
			}

			run.returns = append(run.returns, returnFrame{kind: returnNode, resume: item.resume})
			current = item.child
		case transitionArraySequence:
			count, countErr := run.decodeCount(item.minimum, item.maximum)
			if countErr != nil {
				return jsonvalue.Value{}, countErr
			}

			if err := run.checkArraySequence(count, program.completion[item.child].cost); err != nil {
				return jsonvalue.Value{}, err
			}

			if err := run.begin(constructionArray, '['); err != nil {
				return jsonvalue.Value{}, err
			}

			if count == 0 {
				if err := run.appendOutput([]byte{']'}); err != nil {
					return jsonvalue.Value{}, err
				}

				run.constructions = run.constructions[:len(run.constructions)-1]

				next, complete, returnErr := run.completeValue()
				if returnErr != nil {
					return jsonvalue.Value{}, returnErr
				}

				if complete {
					return run.finish()
				}

				current = next

				continue
			}

			if err := run.beginElement(constructionArray, nil); err != nil {
				return jsonvalue.Value{}, err
			}

			run.returns = append(run.returns, returnFrame{
				kind: returnArraySequence, child: item.child, remaining: count - 1,
			})
			current = item.child
		case transitionObjectMember:
			if err := run.beginElement(constructionObject, item.nameJSON); err != nil {
				return jsonvalue.Value{}, err
			}

			run.returns = append(run.returns, returnFrame{kind: returnNode, resume: item.resume})
			current = item.child
		case transitionStop:
			if len(run.constructions) == 0 {
				return jsonvalue.Value{}, fmt.Errorf("stop has no open container")
			}

			frame := run.constructions[len(run.constructions)-1]

			closing := byte(']')
			if frame.kind == constructionObject {
				closing = '}'
			} else if frame.kind != constructionArray {
				return jsonvalue.Value{}, fmt.Errorf("container stop reached an open string")
			}

			if err := run.appendOutput([]byte{closing}); err != nil {
				return jsonvalue.Value{}, err
			}

			run.constructions = run.constructions[:len(run.constructions)-1]

			next, complete, returnErr := run.completeValue()
			if returnErr != nil {
				return jsonvalue.Value{}, returnErr
			}

			if complete {
				return run.finish()
			}

			current = next
		default:
			return jsonvalue.Value{}, fmt.Errorf("sealed transition %d has unknown action %d", transitionID, item.kind)
		}
	}
}

func (run *execution) chargeStep() error {
	if run.steps == ^uint64(0) {
		return &LimitError{Resource: "steps", Limit: run.limits.MaxSteps, Observed: ^uint64(0)}
	}

	run.steps++

	return checkLimit("steps", run.limits.MaxSteps, run.steps)
}

func (run *execution) appendOutput(value []byte) error {
	current := uint64(len(run.output))
	if uint64(len(value)) > ^uint64(0)-current {
		return &LimitError{
			Resource: "output bytes", Limit: run.limits.MaxOutputBytes, Observed: ^uint64(0),
		}
	}

	observed := current + uint64(len(value))
	if err := checkLimit("output bytes", run.limits.MaxOutputBytes, observed); err != nil {
		return err
	}

	run.output = append(run.output, value...)

	return nil
}

func (run *execution) checkValueDepth(valueDepth uint64) error {
	base := uint64(len(run.constructions))
	if valueDepth > ^uint64(0)-base {
		return &LimitError{Resource: "depth", Limit: run.limits.MaxDepth, Observed: ^uint64(0)}
	}

	return checkLimit("depth", run.limits.MaxDepth, base+valueDepth)
}

func (run *execution) begin(kind constructionKind, opening byte) error {
	if err := run.checkValueDepth(1); err != nil {
		return err
	}

	if err := run.appendOutput([]byte{opening}); err != nil {
		return err
	}

	run.constructions = append(run.constructions, constructionFrame{kind: kind})

	return nil
}

func (run *execution) ensureString() error {
	if len(run.constructions) != 0 && run.constructions[len(run.constructions)-1].kind == constructionString {
		return nil
	}

	return run.begin(constructionString, '"')
}

func (run *execution) beginElement(kind constructionKind, name []byte) error {
	if len(run.constructions) == 0 || run.constructions[len(run.constructions)-1].kind != kind {
		return fmt.Errorf("element action does not match open container")
	}

	frame := &run.constructions[len(run.constructions)-1]
	if frame.count != 0 {
		if err := run.appendOutput([]byte{','}); err != nil {
			return err
		}
	}

	if kind == constructionObject {
		if err := run.appendOutput(name); err != nil {
			return err
		}

		if err := run.appendOutput([]byte{':'}); err != nil {
			return err
		}
	}

	if frame.count == ^uint64(0) {
		return &LimitError{Resource: "steps", Limit: run.limits.MaxSteps, Observed: ^uint64(0)}
	}

	frame.count++

	return nil
}

func (run *execution) completeValue() (NodeID, bool, error) {
	for len(run.returns) != 0 {
		last := len(run.returns) - 1
		frame := run.returns[last]
		run.returns = run.returns[:last]

		switch frame.kind {
		case returnNode:
			return frame.resume, false, nil
		case returnArraySequence:
			if frame.remaining != 0 {
				if err := run.chargeStep(); err != nil {
					return 0, false, err
				}

				if err := run.beginElement(constructionArray, nil); err != nil {
					return 0, false, err
				}

				frame.remaining--
				run.returns = append(run.returns, frame)

				return frame.child, false, nil
			}

			if err := run.appendOutput([]byte{']'}); err != nil {
				return 0, false, err
			}

			run.constructions = run.constructions[:len(run.constructions)-1]
		default:
			return 0, false, fmt.Errorf("unknown return frame %d", frame.kind)
		}
	}

	return 0, true, nil
}

func (run *execution) finish() (jsonvalue.Value, error) {
	if len(run.constructions) != 0 || len(run.returns) != 0 {
		return jsonvalue.Value{}, fmt.Errorf("program returned with unfinished construction")
	}

	value, err := jsonvalue.Parse(run.output)
	if err != nil {
		return jsonvalue.Value{}, fmt.Errorf("program emitted invalid JSON: %w", err)
	}

	return value, nil
}

func (run *execution) decodeCount(minimum *big.Int, maximum *big.Int) (uint64, error) {
	selected := new(big.Int).Set(minimum)
	if maximum == nil || minimum.Cmp(maximum) != 0 {
		natural, err := run.reader.natural(run.chargeStep)
		if err != nil {
			return 0, err
		}

		if maximum == nil {
			selected.Add(selected, natural)
		} else {
			width := new(big.Int).Sub(maximum, minimum)
			width.Add(width, big.NewInt(1))
			selected.Add(selected, natural.Mod(natural, width))
		}
	}

	if !selected.IsUint64() {
		return 0, &LimitError{
			Resource: "steps", Limit: run.limits.MaxSteps, Observed: ^uint64(0),
		}
	}

	return selected.Uint64(), nil
}

//nolint:mnd // Two brackets and one separator per item are the JSON structural lower bound.
func (run *execution) checkArraySequence(count uint64, childCost uint64) error {
	remainingSteps := uint64(0)
	if run.steps <= run.limits.MaxSteps {
		remainingSteps = run.limits.MaxSteps - run.steps
	}

	perItemSteps := new(big.Int).SetUint64(childCost)
	perItemSteps.Add(perItemSteps, big.NewInt(1))

	requiredSteps := new(big.Int).SetUint64(count)
	requiredSteps.Mul(requiredSteps, perItemSteps)

	if count != 0 {
		requiredSteps.Sub(requiredSteps, big.NewInt(1))
	}

	if requiredSteps.Cmp(new(big.Int).SetUint64(remainingSteps)) > 0 {
		observed := run.limits.MaxSteps + 1
		if observed == 0 {
			observed = ^uint64(0)
		}

		return &LimitError{Resource: "steps", Limit: run.limits.MaxSteps, Observed: observed}
	}

	minimumBytes := new(big.Int).SetUint64(count)
	minimumBytes.Mul(minimumBytes, big.NewInt(2))

	if count == 0 {
		minimumBytes.SetUint64(2)
	} else {
		minimumBytes.Add(minimumBytes, big.NewInt(1))
	}

	availableBytes := uint64(0)
	if uint64(len(run.output)) <= run.limits.MaxOutputBytes {
		availableBytes = run.limits.MaxOutputBytes - uint64(len(run.output))
	}

	if minimumBytes.Cmp(new(big.Int).SetUint64(availableBytes)) > 0 {
		observed := run.limits.MaxOutputBytes + 1
		if observed == 0 {
			observed = ^uint64(0)
		}

		return &LimitError{
			Resource: "output bytes", Limit: run.limits.MaxOutputBytes, Observed: observed,
		}
	}

	return run.checkValueDepth(1)
}

func (run *execution) decodeInteger(minimum *big.Int, maximum *big.Int) ([]byte, error) {
	natural, err := run.reader.natural(run.chargeStep)
	if err != nil {
		return nil, err
	}

	value := new(big.Int)

	switch {
	case minimum != nil && maximum != nil:
		width := new(big.Int).Sub(maximum, minimum)
		width.Add(width, big.NewInt(1))
		value.Mod(natural, width)
		value.Add(value, minimum)
	case minimum != nil:
		value.Add(minimum, natural)
	case maximum != nil:
		value.Sub(maximum, natural)
	default:
		value.Rsh(new(big.Int).Set(natural), 1)

		if natural.Bit(0) != 0 {
			value.Add(value, big.NewInt(1))
			value.Neg(value)
		}
	}

	return []byte(value.String()), nil
}

func (program *Program) chooseTransition(outgoing []uint32, reader *tapeReader) (uint32, error) {
	if len(outgoing) == 1 {
		return outgoing[0], nil
	}

	total := uint64(0)

	for _, transitionID := range outgoing {
		weight := uint64(program.transitions[transitionID].weight)
		if weight > ^uint64(0)-total {
			return 0, fmt.Errorf("sampling weight total overflows uint64")
		}

		total += weight
	}

	selected, _ := bits.Mul64(reader.word(), total)

	for _, transitionID := range outgoing {
		weight := uint64(program.transitions[transitionID].weight)
		if selected < weight {
			return transitionID, nil
		}

		selected -= weight
	}

	return 0, fmt.Errorf("sampling interval does not select a transition")
}

func chooseScalar(ranges []ScalarRange, reader *tapeReader) rune {
	total := uint64(0)
	for _, item := range ranges {
		total += uint64(item.Last-item.First) + 1
	}

	selected := uint64(0)
	if total > 1 {
		selected = reader.word() % total
	}

	for _, item := range ranges {
		width := uint64(item.Last-item.First) + 1
		if selected < width {
			return item.First + rune(selected)
		}

		selected -= width
	}

	panic("sealed scalar ranges are empty")
}
