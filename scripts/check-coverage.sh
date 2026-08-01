#!/bin/sh
set -eu

manifest=coverage-exceptions.txt
expected_reason='encodeString rejects invalid UTF-8 first, and encoding/json does not return an error when marshaling a valid Go string; the error is still explicitly propagated.'
expected_statement='return fmt.Errorf("encode JSON string: %w", err)'

if [ ! -f "$manifest" ]; then
	echo "coverage exception manifest is missing: $manifest" >&2
	exit 1
fi

manifest_lines=$(wc -l < "$manifest" | tr -d ' ')
if [ "$manifest_lines" -ne 1 ]; then
	echo "coverage exception manifest must contain exactly one row" >&2
	exit 1
fi

IFS="	" read -r exception_region exception_statements exception_function exception_statement exception_reason < "$manifest"
if [ -z "$exception_region" ] || [ "$exception_statements" != 1 ] || \
	[ "$exception_function" != encodeString ] || \
	[ "$exception_statement" != "$expected_statement" ] || \
	[ "$exception_reason" != "$expected_reason" ]; then
	echo "coverage exception manifest row has an invalid shape or content" >&2
	exit 1
fi

exception_file=${exception_region%%:*}
exception_coordinates=${exception_region#*:}
case "$exception_file" in
	pkg/jsonvalue/value.go) ;;
	*)
		echo "coverage exception must identify pkg/jsonvalue/value.go" >&2
		exit 1
		;;
esac
case "$exception_coordinates" in
	[0-9]*.[0-9]*,[0-9]*.[0-9]*) ;;
	*)
		echo "coverage exception must identify one exact Go cover-profile region" >&2
		exit 1
		;;
esac

start_line=${exception_coordinates%%.*}
end_position=${exception_coordinates#*,}
end_line=${end_position%%.*}
source_region=$(awk -v start="$start_line" -v end="$end_line" 'NR >= start && NR <= end {print}' "$exception_file")
if ! printf '%s\n' "$source_region" | awk -v expected="$expected_statement" '
	{sub(/^[[:space:]]*/, "")}
	$0 == expected {found = 1}
	END {exit !found}
'; then
	echo "coverage exception statement text or coordinates drifted" >&2
	exit 1
fi
if ! awk -v target="$start_line" '
	NR <= target && /^func encodeString\(/ {function_line = NR}
	NR <= target && /^func / && !/^func encodeString\(/ {if (NR > function_line) function_line = 0}
	END {exit function_line == 0}
' "$exception_file"; then
	echo "coverage exception function or coordinates drifted" >&2
	exit 1
fi

module=$(go list -m -f '{{.Path}}')
packages=$(go list -f '{{if or .GoFiles .CgoFiles}}{{.ImportPath}}{{end}}' ./pkg/... | LC_ALL=C sort -u)
if [ -z "$packages" ]; then
	echo "no production packages found under ./pkg/..." >&2
	exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
uncovered="$work/uncovered"
: > "$uncovered"

printf '%-70s %10s %10s %10s %10s\n' PACKAGE TOTAL COVERED UNCOVERED COVERAGE
for package in $packages; do
	profile="$work/$(printf '%s' "$package" | tr '/.' '__').cover"
	go test \
		-count=1 \
		-shuffle=off \
		-run '^(Test|Example)' \
		-covermode=atomic \
		-coverpkg="$package" \
		-coverprofile="$profile" \
		"$package" >/dev/null

	awk -v module="$module" -v output="$uncovered" '
		NR == 1 {next}
		{
			total += $2
			if ($3 > 0) {
				covered += $2
			} else {
				region = $1
				prefix = module "/"
				if (index(region, prefix) == 1) {
					region = substr(region, length(prefix) + 1)
				}
				print region "\t" $2 >> output
			}
		}
		END {
			uncovered = total - covered
			percentage = total == 0 ? 100 : covered * 100 / total
			printf "%-70s %10d %10d %10d %9.2f%%\n", package, total, covered, uncovered, percentage
		}
	' package="$package" "$profile"
done

uncovered_rows=$(wc -l < "$uncovered" | tr -d ' ')
if [ "$uncovered_rows" -ne 1 ]; then
	echo "expected exactly one uncovered production region, found $uncovered_rows" >&2
	cat "$uncovered" >&2
	exit 1
fi

IFS="	" read -r actual_region actual_statements < "$uncovered"
if [ "$actual_region" != "$exception_region" ] || [ "$actual_statements" != "$exception_statements" ]; then
	echo "uncovered production statement does not match the exact manifest row" >&2
	printf 'manifest: %s statements=%s\n' "$exception_region" "$exception_statements" >&2
	printf 'actual:   %s statements=%s\n' "$actual_region" "$actual_statements" >&2
	exit 1
fi

printf 'coverage exception matched exactly: %s (%s statement)\n' "$actual_region" "$actual_statements"
