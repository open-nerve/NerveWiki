import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { RouteObject } from "react-router";
import { expect, test } from "vitest";

import { SignedIn } from "../app/guards";
import { onboardingSteps, type OnboardingStep } from "../onboarding/steps";
import type { User } from "../services/account.service";
import { json, problem, signedInApp, userJSON, type Answer } from "../test/fakes";
import { renderApp } from "../test/render";
import { Onboarding } from "./onboarding";

const newAccount: User = { ...userJSON, display_name: "ada", onboarding_steps: [] };

/**
 * A signed-in tab whose account the fake server keeps and changes: what
 * changed it is in sent; while stepsDown, recording a step answers 503.
 */
function accountServer(routes: Record<string, Answer> = {}) {
  const server = { sent: [] as string[], stepsDown: false };
  let me = newAccount;
  const app = signedInApp({
    "GET /api/v0/me": () => json(me),
    "PATCH /api/v0/me": async (request) => {
      const changes = (await request.json()) as Partial<User>;
      server.sent.push(`name ${changes.display_name}`);
      me = { ...me, ...changes };
      return json(me);
    },
    "POST /api/v0/me/onboarding-steps": async (request) => {
      if (server.stepsDown) {
        return problem(503, "server_busy");
      }
      const { step } = (await request.json()) as { step: string };
      server.sent.push(`step ${step}`);
      me = { ...me, onboarding_steps: [...me.onboarding_steps, step] };
      return json(me);
    },
    ...routes,
  });
  return { app, server };
}

/** Onboarding with the profile step and a second one of the test's. */
function Confirm({ complete }: { complete: () => Promise<void> }) {
  return (
    <button type="button" onClick={() => void complete()}>
      Confirm
    </button>
  );
}
const twoSteps: OnboardingStep[] = [
  ...onboardingSteps.filter((step) => step.id === "profile"),
  { id: "confirm", title: "onboarding.profile.title", Component: Confirm },
];
const routes: RouteObject[] = [
  {
    Component: SignedIn,
    children: [
      { path: "onboarding", Component: () => <Onboarding steps={twoSteps} /> },
      { path: "*", Component: () => <p>elsewhere</p> },
    ],
  },
];

test("each completed step is recorded and the next shows; after the last, the tab goes to next", async () => {
  const user = userEvent.setup();
  const { app, server } = accountServer();
  const { router } = renderApp("/onboarding?next=%2Facme", app, { routes });
  expect(await screen.findByText("Step 1 of 2")).toBeTruthy();
  const name = await screen.findByLabelText("Display name");
  expect(name).toHaveProperty("value", "ada");

  await user.clear(name);
  await user.type(name, " Ada Lovelace ");
  await user.click(screen.getByRole("button", { name: "Continue" }));
  expect(await screen.findByText("Step 2 of 2")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Confirm" }));

  expect(await screen.findByText("elsewhere")).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme");
  expect(server.sent).toEqual(["name Ada Lovelace", "step profile", "step confirm"]);
});

test("a name left as it was is not sent again", async () => {
  const user = userEvent.setup();
  const { app, server } = accountServer();
  renderApp("/onboarding", app, { routes });

  await user.click(await screen.findByRole("button", { name: "Continue" }));

  expect(await screen.findByText("Step 2 of 2")).toBeTruthy();
  expect(server.sent).toEqual(["step profile"]);
});

test("a refused name shows under the field, and the step stays", async () => {
  const user = userEvent.setup();
  const { app, server } = accountServer({
    "PATCH /api/v0/me": () =>
      problem(422, "validation_failed", { errors: [{ field: "display_name", code: "too_long" }] }),
  });
  renderApp("/onboarding", app, { routes });

  await user.type(await screen.findByLabelText("Display name"), "!");
  await user.click(screen.getByRole("button", { name: "Continue" }));

  expect(await screen.findByText("At most 100 characters.")).toBeTruthy();
  expect(screen.getByText("Step 1 of 2")).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
  expect(server.sent).toEqual([]);
});

test("a step that cannot be recorded says why and stays; Continue records it then", async () => {
  const user = userEvent.setup();
  const { app, server } = accountServer();
  server.stepsDown = true;
  renderApp("/onboarding", app, { routes });
  await user.type(await screen.findByLabelText("Display name"), "!");

  await user.click(screen.getByRole("button", { name: "Continue" }));
  expect((await screen.findByRole("alert")).textContent).toBe("The server is busy. Try again in a moment.");
  expect(screen.getByText("Step 1 of 2")).toBeTruthy();

  server.stepsDown = false;
  await user.click(screen.getByRole("button", { name: "Continue" }));

  expect(await screen.findByText("Step 2 of 2")).toBeTruthy();
  expect(server.sent).toEqual(["name ada!", "step profile"]);
});
