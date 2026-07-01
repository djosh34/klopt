package testgenerator

import "encoding/json"

type Caseable interface {
	ValidCases() []Case
	InvalidCases() []Case
	Merge(SchemaNode) (SchemaNode, error)
}

type Case struct {
	Name  string
	Value json.RawMessage
}
