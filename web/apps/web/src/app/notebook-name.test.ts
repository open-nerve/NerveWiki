import { expect, test } from "vitest";

import { notebookNameProblem } from "./notebook-name";

// The bytes are counted in NFC once trimmed, as the server stores the name:
// "é" decomposed is 3 bytes, composed 2.
test.each([
  ["Plans", undefined],
  ["  研发 周报 ", undefined],
  [" \t ", "field.required"],
  ["a".repeat(255), undefined],
  [` ${"a".repeat(255)} `, undefined],
  ["a".repeat(256), "field.notebook_name.too_long"],
  ["研".repeat(85), undefined],
  ["研".repeat(86), "field.notebook_name.too_long"],
  ["é".repeat(127), undefined],
  ["é".repeat(128), "field.notebook_name.too_long"],
  ["a/b", undefined],
])("the name %j: %s", (name, want) => {
  expect(notebookNameProblem(name)).toBe(want);
});
