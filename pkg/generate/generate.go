package generate

func (c *GenerateContext) Generate(dir string) error {
	operations, err := c.JSONRequestBodySchemaObjects()
	if err != nil {
		return err
	}

	c.Operations = operations

	return nil
}
