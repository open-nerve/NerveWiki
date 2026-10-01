import { expect, test } from "vitest";

import { slugFrom, slugProblem, workspaceNameProblem } from "./slug";

test.each([
  ["acme", undefined],
  ["a", undefined],
  ["acme_labs-2", undefined],
  ["a".repeat(48), undefined],
  ["", "field.required"],
  ["a".repeat(49), "field.slug.invalid_format"],
  ["Acme", "field.slug.invalid_format"],
  [" acme", "field.slug.invalid_format"],
  ["acme labs", "field.slug.invalid_format"],
  ["acmé", "field.slug.invalid_format"],
  ["研发", "field.slug.invalid_format"],
])("the slug %j: %s", (slug, want) => {
  expect(slugProblem(slug)).toBe(want);
});

test.each([
  ["Acme", undefined],
  ["  Acme 研发 ", undefined],
  ["  ", "field.required"],
  ["a".repeat(80), undefined],
  [` ${"a".repeat(80)} `, undefined],
  ["a".repeat(81), "field.workspace_name.too_long"],
  ["😀".repeat(80), undefined],
  ["😀".repeat(81), "field.workspace_name.too_long"],
])("the name %j: %s", (name, want) => {
  expect(workspaceNameProblem(name)).toBe(want);
});

test.each([
  ["Acme", "acme"],
  ["Acme Labs", "acme-labs"],
  ["  Acme - Labs!  ", "acme-labs"],
  ["research_and_dev 2", "research_and_dev-2"],
  ["Café Crème", "cafe-creme"],
  ["研发部", ""],
  ["Acme 研发", "acme"],
  [`${"a".repeat(47)} b`, "a".repeat(47)],
  ["a".repeat(60), "a".repeat(48)],
])("the name %j suggests %j", (name, slug) => {
  expect(slugFrom(name)).toBe(slug);
  if (slug !== "") {
    expect(slugProblem(slug)).toBeUndefined();
  }
});
