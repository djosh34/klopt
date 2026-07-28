//nolint:godoclint // Private schema-generation catalog entries are intentionally concise.
package testgenerator

type opaqueStringFragment struct {
	Pattern string
	Format  string
}

var opaqueStringCatalog = []opaqueStringFragment{
	{Pattern: `^C[0-9]{3}$`},
	{Pattern: `^u[0-9]{3}@example[.]com$`, Format: "email"},
	{Pattern: `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`, Format: "date"},
	{Pattern: `^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`, Format: "date-time"},
	{Pattern: `^[A-Za-z0-9+/]+={0,2}$`, Format: "byte"},
}
