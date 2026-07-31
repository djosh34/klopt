//nolint:godoclint // Focused fault plans are private generator state.
package testgenerator

import (
	"errors"
	"fmt"
	"slices"
	"sort"
)

type faultTargetKind uint8

const (
	faultAtom faultTargetKind = iota
	faultAll
	faultAny
)

type faultPathStepKind uint8

const (
	faultProperty faultPathStepKind = iota
	faultArrayItem
	faultAdditionalProperty
)

type faultPathStep struct {
	kind     faultPathStepKind
	property string
}

type faultPlan struct {
	target      *expression
	targetKind  faultTargetKind
	schemaPath  []faultPathStep
	failedChild int
}

type faultState uint8

const (
	faultPending faultState = iota
	faultInjected
)

type faultToken struct {
	plan  *faultPlan
	state faultState
}

func newFaultToken(plan *faultPlan) *faultToken {
	return &faultToken{plan: plan, state: faultPending}
}

func newAtomFaultPlan(target *expression, path []faultPathStep) faultPlan {
	return faultPlan{
		target: target, targetKind: faultAtom,
		schemaPath: slices.Clone(path), failedChild: -1,
	}
}

func newAllFaultPlan(target *expression, path []faultPathStep, failedChild int) faultPlan {
	return faultPlan{
		target: target, targetKind: faultAll,
		schemaPath: slices.Clone(path), failedChild: failedChild,
	}
}

func newAnyFaultPlan(target *expression, path []faultPathStep) faultPlan {
	return faultPlan{
		target: target, targetKind: faultAny,
		schemaPath: slices.Clone(path), failedChild: -1,
	}
}

func (token *faultToken) markInjected() error {
	if token == nil {
		return errors.New("mark fault on nil token")
	}

	if token.state != faultPending {
		return errors.New("focused fault injected more than once")
	}

	token.state = faultInjected

	return nil
}

func enumerateFaultPlans(root *expression) ([]faultPlan, error) {
	if root == nil {
		return nil, errors.New("enumerate faults from nil root")
	}

	plans := make([]faultPlan, 0)
	if err := appendFaultPlans(root, nil, &plans); err != nil {
		return nil, err
	}

	return plans, nil
}

//nolint:cyclop,gocognit // Fault enumeration follows the closed expression and path model.
func appendFaultPlans(
	rootExpression *expression,
	path []faultPathStep,
	plans *[]faultPlan,
) error {
	if rootExpression == nil {
		return errors.New("append faults from nil expression")
	}

	if plans == nil {
		return errors.New("append faults with nil plan output")
	}

	active := make(map[*expression]bool)

	var appendPlans func(*expression, []faultPathStep) error

	appendPlans = func(current *expression, currentPath []faultPathStep) error {
		if current == nil {
			return errors.New("fault plan contains nil expression")
		}

		if active[current] {
			return fmt.Errorf("fault plan expression cycle at %p", current)
		}

		active[current] = true
		defer delete(active, current)

		switch current.kind {
		case expressionAtom:
			if atomCanFail(current.atom) {
				*plans = append(*plans, newAtomFaultPlan(current, currentPath))
			}

			return appendStructuralPlans(current, currentPath, &appendPlans)
		case expressionAll:
			if len(current.children) == 0 {
				return nil
			}

			for childIndex, child := range current.children {
				if child == nil {
					return fmt.Errorf("all child %d is nil", childIndex)
				}

				*plans = append(*plans, newAllFaultPlan(current, currentPath, childIndex))
			}

			for _, child := range current.children {
				if err := appendPlans(child, currentPath); err != nil {
					return err
				}
			}

			return nil
		case expressionAny:
			if len(current.children) == 0 {
				return nil
			}

			*plans = append(*plans, newAnyFaultPlan(current, currentPath))

			return nil
		default:
			return fmt.Errorf("unknown expression kind %d", current.kind)
		}
	}

	return appendPlans(rootExpression, slices.Clone(path))
}

//nolint:cyclop // Structural descent is deterministic and ordered by schema route.
func appendStructuralPlans(
	currentExpression *expression,
	path []faultPathStep,
	appendPlans *func(*expression, []faultPathStep) error,
) error {
	properties := make([]struct {
		name  string
		child *expression
	}, 0)

	items := make([]*expression, 0)
	additional := make([]*expression, 0)

	for _, child := range expressionChildren(currentExpression) {
		if child == nil || child.kind != expressionAtom {
			continue
		}

		rule := child.atom
		switch rule.kind {
		case atomObjectProperty:
			if rule.child == nil {
				return fmt.Errorf("property %q has nil child", rule.name)
			}

			properties = append(properties, struct {
				name  string
				child *expression
			}{name: rule.name, child: rule.child})
		case atomArrayItems:
			if rule.child == nil {
				return errors.New("array items has nil child")
			}

			items = append(items, rule.child)
		case atomObjectAdditional:
			if rule.hasChild {
				if rule.child == nil {
					return errors.New("additional properties has nil child")
				}

				additional = append(additional, rule.child)
			}
		}
	}

	sort.Slice(properties, func(left, right int) bool {
		return properties[left].name < properties[right].name
	})

	for _, property := range properties {
		childPath := append(slices.Clone(path), faultPathStep{
			kind: faultProperty, property: property.name,
		})
		if err := (*appendPlans)(property.child, childPath); err != nil {
			return err
		}
	}

	for _, item := range items {
		if err := (*appendPlans)(item, append(slices.Clone(path), faultPathStep{
			kind: faultArrayItem,
		})); err != nil {
			return err
		}
	}

	for _, child := range additional {
		if err := (*appendPlans)(child, append(slices.Clone(path), faultPathStep{
			kind: faultAdditionalProperty,
		})); err != nil {
			return err
		}
	}

	return nil
}

func expressionChildren(currentExpression *expression) []*expression {
	if currentExpression.kind != expressionAtom {
		return currentExpression.children
	}

	return []*expression{currentExpression}
}

//nolint:cyclop // Every retained atom kind has one explicit failure rule.
func atomCanFail(rule atom) bool {
	switch rule.kind {
	case atomKinds:
		for _, allowed := range rule.allowed {
			if !allowed {
				return true
			}
		}

		return false
	case atomEnum:
		return len(rule.values) > 0
	case atomArrayItems:
		return rule.child != nil
	case atomObjectRequired:
		return len(rule.names) > 0
	case atomObjectProperty:
		return rule.child != nil
	case atomObjectAdditional:
		return rule.hasChild && rule.child != nil || !rule.allowedAdditional
	default:
		return true
	}
}
