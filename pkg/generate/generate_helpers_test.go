package generate

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	// fileURIWithPositionRegex matches template file URIs with line and column positions.
	fileURIWithPositionRegex = regexp.MustCompile(`file://\S+?:\d+:\d+`)
	// relativeFileURIPrefixRegex matches template file URIs with relative paths.
	relativeFileURIPrefixRegex = regexp.MustCompile(`file://([^/])`)
)

const (
	// ansiRed starts red diff output.
	ansiRed = "\x1b[31m"
	// ansiGreen starts green diff output.
	ansiGreen = "\x1b[32m"
	// ansiCyan starts cyan diff output.
	ansiCyan = "\x1b[36m"
	// ansiReset resets diff output color.
	ansiReset = "\x1b[0m"
)

// GenerateWithPathError supports generator tests.
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
	generateErr = wrapTemplateError(generateErr)

	return transformError(generateErr, func(errorString string) string {
		return normalizeFileURIBlocks(errorString, absolutePathPrefix)
	})
}

// wrapTemplateError supports generator tests.
func wrapTemplateError(templateErr error) error {
	matches, err := fs.Glob(templateFS, templatePattern)
	if err != nil {
		return errors.Join(templateErr, err)
	}

	replacements := make([]string, 0, len(matches)*2)
	for _, match := range matches {
		replacements = append(replacements, path.Base(match), fmt.Sprintf("file://%s", match))
	}

	replacer := strings.NewReplacer(replacements...)

	return transformError(templateErr, replacer.Replace)
}

// transformedWrappedError supports generator tests.
type transformedWrappedError struct {
	message string
	err     error
}

// Error supports generator tests.
func (e transformedWrappedError) Error() string {
	return e.message
}

// Unwrap supports generator tests.
func (e transformedWrappedError) Unwrap() error {
	return e.err
}

// transformError supports generator tests.
func transformError(err error, transform func(string) string) error {
	if err == nil {
		return nil
	}

	if unwrapper, ok := err.(interface{ Unwrap() []error }); ok {
		unwrappedErrors := unwrapper.Unwrap()

		transformedErrors := make([]error, 0, len(unwrappedErrors))
		for _, unwrappedError := range unwrappedErrors {
			transformedErrors = append(transformedErrors, transformError(unwrappedError, transform))
		}

		joined := errors.Join(transformedErrors...)
		if joined != nil {
			return joined
		}
	}

	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		unwrappedError := unwrapper.Unwrap()
		if unwrappedError != nil {
			return transformedWrappedError{
				message: transform(err.Error()),
				err:     transformError(unwrappedError, transform),
			}
		}
	}

	return errors.New(transform(err.Error()))
}

// TestNormalizeFileURIBlocks exercises the named generator behavior.
func TestNormalizeFileURIBlocks(t *testing.T) {
	t.Parallel()

	errorString := `template: file://templates/file.go.tmpl:22:3: ` +
		`executing "file://templates/file.go.tmpl" at <.Generate>: error calling Generate: ` +
		`template: file://templates/string.go.tmpl:1:13: ` +
		`executing "file://templates/string.go.tmpl" at <.Name>`
	expected := `template: file:///repo/pkg/generate/templates/file.go.tmpl:22:3` + "\n" +
		`: executing "templates/file.go.tmpl" at <.Generate>: error calling Generate: ` +
		`template: file:///repo/pkg/generate/templates/string.go.tmpl:1:13` + "\n" +
		`: executing "templates/string.go.tmpl" at <.Name>`

	require.Equal(t, expected, normalizeFileURIBlocks(errorString, "file:///repo/pkg/generate/"))
}

// TestTransformErrorPreservesUnwraps exercises the named generator behavior.
func TestTransformErrorPreservesUnwraps(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("outer file.go.tmpl: %w", errors.New("inner string.go.tmpl"))
	joined := errors.Join(errors.New("joined array.go.tmpl"), wrapped)

	transformed := transformError(joined, func(errorString string) string {
		return strings.ReplaceAll(errorString, ".go.tmpl", ".go")
	})

	require.Equal(t, "joined array.go\nouter file.go: inner string.go", transformed.Error())

	multiUnwrapper, ok := transformed.(interface{ Unwrap() []error })
	require.True(t, ok)

	unwrappedErrors := multiUnwrapper.Unwrap()
	require.Len(t, unwrappedErrors, 2)
	require.Equal(t, "joined array.go", unwrappedErrors[0].Error())
	require.Equal(t, "outer file.go: inner string.go", unwrappedErrors[1].Error())
	require.Equal(t, "inner string.go", errors.Unwrap(unwrappedErrors[1]).Error())
}

// TestWrapTemplateErrorPreservesUnwraps exercises the named generator behavior.
func TestWrapTemplateErrorPreservesUnwraps(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf(
		"template: object.go.tmpl:5:6: %w",
		errors.New("template: string.go.tmpl:1:13"),
	)
	joined := errors.Join(errors.New("template: file.go.tmpl:22:3"), wrapped)

	templateErr := wrapTemplateError(joined)

	expected := "template: file://templates/file.go.tmpl:22:3\n" +
		"template: file://templates/object.go.tmpl:5:6: " +
		"template: file://templates/string.go.tmpl:1:13"
	require.Equal(t, expected, templateErr.Error())

	multiUnwrapper, ok := templateErr.(interface{ Unwrap() []error })
	require.True(t, ok)

	unwrappedErrors := multiUnwrapper.Unwrap()
	require.Len(t, unwrappedErrors, 2)
	require.Equal(t, "template: file://templates/file.go.tmpl:22:3", unwrappedErrors[0].Error())
	require.Equal(
		t,
		"template: file://templates/object.go.tmpl:5:6: "+
			"template: file://templates/string.go.tmpl:1:13",
		unwrappedErrors[1].Error(),
	)
	require.Equal(t, "template: file://templates/string.go.tmpl:1:13", errors.Unwrap(unwrappedErrors[1]).Error())
}

// normalizeFileURIBlocks supports generator tests.
func normalizeFileURIBlocks(errorString string, absolutePathPrefix string) string {
	errorString = fileURIWithPositionRegex.ReplaceAllStringFunc(errorString, func(fileURIBlock string) string {
		return strings.Replace(fileURIBlock, "file://", absolutePathPrefix, 1) + "\n"
	})

	return relativeFileURIPrefixRegex.ReplaceAllString(errorString, "$1")
}
