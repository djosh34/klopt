package generate

import (
	"context"

	"github.com/getkin/kin-openapi/openapi3"
)

type GenerateContext struct {
	Document *openapi3.T
}

func (c *GenerateContext) FilterOperations(operation ...string) error {
	// TODO, filter from Document, to keep only the operation provided

	return nil
}

func (c *GenerateContext) Generate(dir string) error {

	return nil
}

func LoadOpenapi(ctx context.Context, path string) (*GenerateContext, error) {
	loader := &openapi3.Loader{Context: ctx, IsExternalRefsAllowed: false}
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	err = doc.Validate(ctx)
	if err != nil {
		return nil, err
	}

	return &GenerateContext{
		Document: doc,
	}, nil
}
