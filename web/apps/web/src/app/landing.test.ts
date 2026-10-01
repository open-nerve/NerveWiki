import { expect, test } from "vitest";

import { landingPath } from "./landing";

const list = [{ slug: "acme" }, { slug: "beta" }, { slug: "zeta" }];

test.each([
  ["the last workspace, when the account still has it", list, "beta", "/beta"],
  ["the first by name, without a last one", list, undefined, "/acme"],
  ["the first by name, when the last is not the account's (any more)", list, "gone", "/acme"],
  ["the page that creates one, without a workspace", [], "acme", "/create-workspace"],
])("/ lands on %s", (_, workspaces, last, want) => {
  expect(landingPath(workspaces, last)).toBe(want);
});
