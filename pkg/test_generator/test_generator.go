package testgenerator

import (
	"encoding/json"
	"fmt"
)

type Caseable interface {
	ValidCases() []Case
	InvalidCases() []Case
}
type Case struct {
	Name  string
	Value json.RawMessage
}

func GenerateCasesFromOpenAPIFile(openapiPath string) ([]Case, error) {
	return nil, fmt.Errorf("generate cases from %q: not implemented", openapiPath)
}
