package generate

import (
	"os"
	"path/filepath"
)

type FileSet map[string][]byte

func (fs FileSet) WriteToDir(dir string) error {
	for name, contents := range fs {
		path := filepath.Join(dir, filepath.FromSlash(name))
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			return err
		}

		err = os.WriteFile(path, contents, 0o644)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *GenerateContext) GenerateInMemory() (FileSet, error) {
	operations, err := c.JSONRequestBodyModelSchemas()
	if err != nil {
		return nil, err
	}

	schemas, err := schemaDefinitions(operations)
	if err != nil {
		return nil, err
	}

	models, err := renderModelsFile(schemas)
	if err != nil {
		return nil, err
	}

	fileSet := make(FileSet)
	fileSet["models.go"] = models

	return fileSet, nil
}

func (c *GenerateContext) Generate(dir string) error {
	err := os.RemoveAll(dir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}

	fileSet, err := c.GenerateInMemory()
	if err != nil {
		return err
	}

	err = fileSet.WriteToDir(dir)
	if err != nil {
		return err
	}

	return nil
}
