# Dynamic Go Execution Benchmark

Date: 2026-06-28

## Baseline: temp file + `go run`

The current implementation writes a generated `main.go` and `go.mod` to a temporary directory, then executes `go run .`.

Command:

```sh
/usr/bin/time -f 'wall_clock=%E max_rss_kb=%M' env RUN_DYNAMIC_GO_BASELINE_1000=1 go test ./pkg/generate -run TestCompileAndRunGoSourceBaseline1000 -count=1 -timeout=30m -v
```

Result:

```text
iterations=1000 total=1m31.439634907s per_iteration=91.439634ms
wall_clock=1:31.64 max_rss_kb=144488
```

Notes:

- This is not in-memory execution.
- Each iteration creates a temp module, writes source files, invokes `go run`, captures stdout/stderr, and removes the temp module.
- The Go build cache helps, but process startup, filesystem work, and toolchain invocation dominate.

## In-memory: Yaegi compile + execute

The in-memory experiment uses `github.com/traefik/yaegi`. It parses and compiles the generated source string into a Yaegi `Program`, then executes that program with stdout/stderr captured in memory.

Command:

```sh
/usr/bin/time -f 'wall_clock=%E max_rss_kb=%M' env RUN_DYNAMIC_GO_IN_MEMORY_1000=1 go test ./pkg/generate -run TestCompileAndRunGoSourceInMemory1000 -count=1 -timeout=30m -v
```

Result:

```text
iterations=1000 total=2.103627947s per_iteration=2.103627ms
wall_clock=0:02.52 max_rss_kb=230308
```

Comparison:

```text
filesystem go run: 91.439634ms/op
in-memory yaegi:    2.103627ms/op
speedup:            ~43.5x
```

Notes:

- This avoids per-iteration generated source files and avoids spawning `go run`.
- It is Go-source-compatible interpretation via Yaegi, not native Go compiler output loaded into the current process.
- Peak RSS was higher in this test because the process loads Yaegi and its stdlib symbol tables.
