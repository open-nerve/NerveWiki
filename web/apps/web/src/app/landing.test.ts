import { expect, test } from "vitest";

import type { Workspace } from "../services/workspace.service";
import { landingPath, notebookTarget } from "./landing";

const list = [{ slug: "acme" }, { slug: "beta" }, { slug: "zeta" }];

test.each([
  ["the last workspace, when the account still has it", list, "beta", "/beta"],
  ["the first by name, without a last one", list, undefined, "/acme"],
  ["the first by name, when the last is not the account's (any more)", list, "gone", "/acme"],
  ["the page that creates one, without a workspace", [], "acme", "/create-workspace"],
])("/ lands on %s", (_, workspaces, last, want) => {
  expect(landingPath(workspaces, last)).toBe(want);
});

const as = (role: Workspace["role"], slug: string) => ({ slug, role });

test.each([
  ["the last workspace, where the account is a member", [as("admin", "acme"), as("member", "beta")], "beta", "beta"],
  [
    "the first by name, when the last is a guest's",
    [as("guest", "acme"), as("member", "beta"), as("guest", "zeta")],
    "zeta",
    "beta",
  ],
  ["the first by name, without a last one", [as("member", "acme"), as("admin", "beta")], undefined, "acme"],
  ["the first by name, past the guest's", [as("guest", "acme"), as("admin", "beta")], "gone", "beta"],
  ["none, a guest everywhere", [as("guest", "acme"), as("guest", "beta")], "acme", undefined],
  ["none, without a workspace", [], "acme", undefined],
])("onboarding creates the first notebook in %s", (_, workspaces, last, want) => {
  expect(notebookTarget(workspaces, last)?.slug).toBe(want);
});
