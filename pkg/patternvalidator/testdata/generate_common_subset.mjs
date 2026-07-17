import { readFileSync, writeFileSync } from "node:fs";

const path = new URL("common_subset.json", import.meta.url);
const fixture = JSON.parse(readFileSync(path, "utf8"));

const resolved = fixture.cases.map(({ pattern, input }) => ({
  pattern,
  input,
  expected: new RegExp(pattern).test(input),
}));

fixture.cases = resolved;
writeFileSync(path, `${JSON.stringify(fixture, null, 2)}\n`);
