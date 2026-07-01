package generate

import (
	"errors"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	fileURIWithPositionRegex = regexp.MustCompile(`file://\S+?:\d+:\d+`)
)

func GenerateWithPathError(t *testing.T, generateContext *GenerateContext, dir string) error {
	t.Helper()

	generateErr := generateContext.Generate(dir)
	if generateErr == nil {
		return nil
	}

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	generateDir, err := filepath.Abs(filepath.Dir(file))
	require.NoError(t, err)

	absolutePathPrefix := "file://" + filepath.ToSlash(generateDir) + "/"

	newErrorString := normalizeFileURIBlocks(generateErr.Error(), absolutePathPrefix)

	return errors.New(newErrorString)
}

func TestNormalizeFileURIBlocks(t *testing.T) {
	errorString := `template: file://templates/file.tmpl:22:3: executing "file://templates/file.tmpl" at <.Generate>: error calling Generate: template: file://templates/string.tmpl:1:13: executing "file://templates/string.tmpl" at <.Name>`

	require.Equal(t, `template: file:///repo/pkg/generate/templates/file.tmpl:22:3
: executing "templates/file.tmpl" at <.Generate>: error calling Generate: template: file:///repo/pkg/generate/templates/string.tmpl:1:13
: executing "templates/string.tmpl" at <.Name>`, normalizeFileURIBlocks(errorString, "file:///repo/pkg/generate/"))
}

func normalizeFileURIBlocks(errorString string, absolutePathPrefix string) string {
	errorString = fileURIWithPositionRegex.ReplaceAllStringFunc(errorString, func(fileURIBlock string) string {
		return strings.Replace(fileURIBlock, "file://", absolutePathPrefix, 1) + "\n"
	})

	return strings.ReplaceAll(errorString, "file://", "")
}
