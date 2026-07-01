package testgenerator

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCasesFromOpenAPIFileNotImplemented(t *testing.T) {
	cases, err := GenerateCasesFromOpenAPIFile("openapi.yaml")
	require.Nil(t, cases)
	require.ErrorContains(t, err, `generate cases from "openapi.yaml": not implemented`)
}

func testdataOpenAPIPath(name string) string {
	return filepath.Join("testdata", name)
}
