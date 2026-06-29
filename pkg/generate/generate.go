package generate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type TemplateStruct struct {
	Name               string
	FunctionEquivelant func()
	TemplateEquivelant string
}

type DynamicGoResult struct {
	Stdout string
	Stderr string
}

func CompileAndRunGoSource(ctx context.Context, source string, args ...string) (DynamicGoResult, error) {
	dir, err := os.MkdirTemp("", "decode-and-validate-dynamic-go-*")
	if err != nil {
		return DynamicGoResult{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module dynamic_go_source\n\ngo 1.26.4\n"), 0o600); err != nil {
		return DynamicGoResult{}, fmt.Errorf("write generated go.mod: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o600); err != nil {
		return DynamicGoResult{}, fmt.Errorf("write generated source: %w", err)
	}

	cmdArgs := append([]string{"run", "."}, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result := DynamicGoResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	if err != nil {
		return result, fmt.Errorf("run generated go source: %w", err)
	}

	return result, nil
}

func CompileAndRunGoSourceInMemory(ctx context.Context, source string, args ...string) (DynamicGoResult, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	argv := append([]string{"dynamic_go_source"}, args...)
	runner := interp.New(interp.Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Args:   argv,
	})

	if err := runner.Use(stdlib.Symbols); err != nil {
		return DynamicGoResult{Stdout: stdout.String(), Stderr: stderr.String()}, fmt.Errorf("load stdlib symbols: %w", err)
	}

	program, err := runner.Compile(source)
	if err != nil {
		return DynamicGoResult{Stdout: stdout.String(), Stderr: stderr.String()}, fmt.Errorf("compile generated source in memory: %w", err)
	}

	if _, err := runner.ExecuteWithContext(ctx, program); err != nil {
		return DynamicGoResult{Stdout: stdout.String(), Stderr: stderr.String()}, fmt.Errorf("execute generated source in memory: %w", err)
	}

	return DynamicGoResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, nil
}
