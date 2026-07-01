package testgenerator

type OpenAPINode struct {
	Paths map[string]struct {
		Post *struct {
			OperationID string `yaml:"operationId"`
			RequestBody struct {
				Required bool `yaml:"required"`
				Content  map[string]struct {
					Schema SchemaNode `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"requestBody"`
		} `yaml:"post"`
	} `yaml:"paths"`
}

type SchemaNode struct {
	Object *ObjectNode
	String *StringNode
}

type BaseNode struct {
	Type     string `yaml:"type"`
	Nullable bool   `yaml:"nullable"`
}
