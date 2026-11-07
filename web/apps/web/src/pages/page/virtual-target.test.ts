import { readFileSync } from "node:fs";
import { URL as FileURL } from "node:url";
import { expect, test } from "vitest";

import { virtualTarget } from "./virtual-target";

type TargetCase = {
  name: string;
  raw: string;
  parents: string[];
  virtual: "index" | "log" | null;
};

// Shared with linking/domain's real WantedTarget test; literal outcomes are
// not obtained from a second implementation in the test.
const cases = JSON.parse(
  readFileSync(new FileURL("../../../../../../testdata/m10/virtual-targets.json", import.meta.url), "utf8")
) as TargetCase[];

test("P4/W03 shared_cases: the fixture has both virtual and ordinary or invalid targets", () => {
  expect(cases).toHaveLength(39);
  expect(cases.filter((row) => row.virtual !== null)).toHaveLength(12);
  expect(cases.filter((row) => row.virtual === null)).toHaveLength(27);
});

test.each(cases)("P4/W03 shared_cases: $name", ({ raw, parents, virtual }) => {
  expect(virtualTarget(raw, parents.length) ?? null).toBe(virtual);
});

test("P4/W03 shared_cases: an unknown ancestry cannot prove a relative root", () => {
  expect(virtualTarget("./index", undefined)).toBeUndefined();
  expect(virtualTarget("../log", undefined)).toBeUndefined();
  expect(virtualTarget("/index", undefined)).toBe("index");
  expect(virtualTarget("log", undefined)).toBe("log");
});
