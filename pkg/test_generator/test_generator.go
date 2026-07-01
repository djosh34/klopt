package testgenerator

import (
	"encoding/json"
	"fmt"
)

type Case struct {
	GenerateValid   func(valid, invalid map[string]SchemaNode) json.RawMessage
	RequiredValid   map[string]SchemaNode
	RequiredInvalid map[string]SchemaNode
}

func GenerateCasesFromOpenAPIFile(openapiPath string) ([]Case, error) {
	return nil, fmt.Errorf("generate cases from %q: not implemented", openapiPath)
}
