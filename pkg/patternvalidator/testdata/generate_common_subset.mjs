import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const path = new URL("common_subset.json", import.meta.url);

if (process.argv[2] === "--evaluate") {
  const cases = JSON.parse(readFileSync(0, "utf8"));
  process.stdout.write(JSON.stringify(cases.map(({ pattern, input }) => new RegExp(pattern).test(input))));
} else {
  const fixture = JSON.parse(readFileSync(path, "utf8"));
  const seed = fixture.metadata.seed;
  const cases = fixture.cases
    .map(({ pattern, input }) => ({ pattern, input }))
    .sort((left, right) => `${left.pattern}\0${left.input}`.localeCompare(`${right.pattern}\0${right.input}`));

  let state = seed >>> 0;
  for (let index = cases.length - 1; index > 0; index -= 1) {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
    const selected = state % (index + 1);
    [cases[index], cases[selected]] = [cases[selected], cases[index]];
  }

  const nodeResults = cases.map(({ pattern, input }) => new RegExp(pattern).test(input));
  const script = fileURLToPath(import.meta.url);
  const bunVersion = execFileSync("bun", ["--version"], { encoding: "utf8" }).trim();
  const bunResults = JSON.parse(execFileSync("bun", [script, "--evaluate"], {
    encoding: "utf8",
    input: JSON.stringify(cases),
  }));

  const disagreements = [];
  const resolved = [];
  for (const [index, test] of cases.entries()) {
    if (nodeResults[index] !== bunResults[index]) {
      disagreements.push({ ...test, node: nodeResults[index], bun: bunResults[index] });
    } else {
      resolved.push({ ...test, expected: nodeResults[index] });
    }
  }

  fixture.metadata = {
    seed,
    nodeVersion: process.version,
    bunVersion,
    generator: "generate_common_subset.mjs",
    disagreements,
  };
  fixture.cases = resolved;
  writeFileSync(path, `${JSON.stringify(fixture, null, 2)}\n`);
}
