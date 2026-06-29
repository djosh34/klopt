package generate

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const generatedHelloSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("generated go says %s\n", os.Args[1])
}
`

func TestCompileAndRunGoSource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := CompileAndRunGoSource(ctx, generatedHelloSource, "hello")
	require.NoError(t, err, result.Stderr)
	require.Equal(t, "generated go says hello\n", result.Stdout)
	require.Empty(t, result.Stderr)
}

func TestCompileAndRunGoSourceInMemory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := CompileAndRunGoSourceInMemory(ctx, generatedHelloSource, "hello")
	require.NoError(t, err, result.Stderr)
	require.Equal(t, "generated go says hello\n", result.Stdout)
	require.Empty(t, result.Stderr)
}

func TestCompileAndRunGoSourceBaseline1000(t *testing.T) {
	if os.Getenv("RUN_DYNAMIC_GO_BASELINE_1000") != "1" {
		t.Skip("set RUN_DYNAMIC_GO_BASELINE_1000=1 to run the exact 1000-iteration baseline")
	}

	start := time.Now()
	for i := 0; i < 1000; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := CompileAndRunGoSource(ctx, generatedHelloSource, "hello")
		cancel()

		require.NoError(t, err, result.Stderr)
		require.Equal(t, "generated go says hello\n", result.Stdout)
		require.Empty(t, result.Stderr)
	}
	elapsed := time.Since(start)
	t.Logf("iterations=1000 total=%s per_iteration=%s", elapsed, elapsed/1000)
}

func TestCompileAndRunGoSourceInMemory1000(t *testing.T) {
	if os.Getenv("RUN_DYNAMIC_GO_IN_MEMORY_1000") != "1" {
		t.Skip("set RUN_DYNAMIC_GO_IN_MEMORY_1000=1 to run the exact 1000-iteration in-memory benchmark")
	}

	start := time.Now()
	for i := 0; i < 1000; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := CompileAndRunGoSourceInMemory(ctx, generatedHelloSource, "hello")
		cancel()

		require.NoError(t, err, result.Stderr)
		require.Equal(t, "generated go says hello\n", result.Stdout)
		require.Empty(t, result.Stderr)
	}
	elapsed := time.Since(start)
	t.Logf("iterations=1000 total=%s per_iteration=%s", elapsed, elapsed/1000)
}
