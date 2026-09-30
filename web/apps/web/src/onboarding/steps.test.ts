import { expect, test } from "vitest";

import { onboardingSteps, pendingSteps, type OnboardingStep } from "./steps";

// The server's rules for a step's id, and how many it records.
test("every step has an id the server records, once, and there are 32 at most", () => {
  const ids = onboardingSteps.map((step) => step.id);

  for (const id of ids) {
    expect(id).toMatch(/^[a-z][a-z0-9_]{0,31}$/);
  }
  expect(new Set(ids).size).toBe(ids.length);
  expect(ids.length).toBeLessThanOrEqual(32);
});

const step = (id: string): OnboardingStep => ({ id, title: "onboarding.profile.title", Component: () => null });
const steps = [step("zeta"), step("alpha"), step("mid")];
const ids = (done: string[]) => pendingSteps({ onboarding_steps: done }, steps).map((s) => s.id);

test("the pending steps are the ones not completed, in the registry's order", () => {
  expect(ids([])).toEqual(["zeta", "alpha", "mid"]);
  expect(ids(["mid", "zeta"])).toEqual(["alpha"]);
  expect(ids(["alpha", "retired_step"])).toEqual(["zeta", "mid"]);
  expect(ids(["mid", "alpha", "zeta"])).toEqual([]);
});
