package testgenerator

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
)

func YAMLBytesToJSONRawMessage(yamlBytes []byte) (*json.RawMessage, error) {
	jsonBytes, err := yaml.YAMLToJSON(yamlBytes)
	if err != nil {
		return nil, fmt.Errorf("convert yaml to json: %w", err)
	}

	rawMessage := json.RawMessage(jsonBytes)

	return &rawMessage, nil
}
