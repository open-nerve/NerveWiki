import { createClient } from "@nervewiki/api-client";

import { accountIdOf, expectOnboardingSteps } from "../../fixtures/assert/identity";
import { expectNewWorkspace } from "../../fixtures/assert/workspace";
import { bearer, createToken, emailFor, register } from "../../fixtures/auth";
import type { Database } from "../../fixtures/db";
import { accept, invite } from "../../fixtures/invitations";
import {
  createFirstNotebookWith,
  notebookStep,
  saveProfileStep,
  stepRecorded,
  workspaceStep,
} from "../../fixtures/onboarding-pages";
import { expect, test } from "../../fixtures/test";
import { createWorkspaceWith, expectCreatePage, nameField, workspaceHeading } from "../../fixtures/workspace-pages";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// W11, onboarding's workspace step (M2 design 3; M2/P5 design 3.7): a new
// account creates a workspace; one that joined by an invitation goes on;
// while creation is off, one without a workspace reads how to get into one.
// The notebook step comes after it (M3/P4 design 3.6; N12 tells it).

/** How many active memberships the account of userId has. */
async function membershipsOf(db: Database, userId: string): Promise<number> {
  const [row] = await db.query<{ n: number }>(
    "SELECT count(*)::int AS n FROM workspace_members WHERE user_id = $1 AND ended_at IS NULL",
    [userId]
  );
  return row?.n ?? 0;
}

test("W11 (API): a token records the workspace step, and creates the workspace", async ({ api, db }, testInfo) => {
  const email = emailFor(testInfo);
  const pat = (await createToken(api, (await register(api, email)).access_token, { name: "W11" })).token;
  const userId = await accountIdOf(db, email);

  const record = (step: string) => api.POST("/api/v0/me/onboarding-steps", { body: { step }, headers: bearer(pat) });
  expect((await record("profile")).response.status).toBe(200);
  expect((await record("workspace")).response.status).toBe(200);
  const created = await createWorkspace(api, pat, "Acme", slugFor(testInfo));

  await expectOnboardingSteps(db, userId, ["profile", "workspace"]);
  await expectNewWorkspace(db, created, userId);
});

test("W11 (page): a new account creates a workspace in onboarding, then its first notebook, then goes into it", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo);
  const page = await signedInPage(await register(api, email));
  const userId = await accountIdOf(db, email);
  await page.goto("/");
  expect(await saveProfileStep(page)).toBe(200);

  await expect(workspaceStep(page)).toBeVisible();
  await expect(page.getByText("Step 2 of 3", { exact: true })).toBeVisible();
  const slug = slugFor(testInfo);
  const recorded = stepRecorded(page, "workspace");
  const { status, created } = await createWorkspaceWith(page, { name: "Acme", slug, button: "Create and continue" });
  expect(status).toBe(201);
  expect((await recorded).status()).toBe(200);
  await expect(notebookStep(page)).toBeVisible();
  expect((await createFirstNotebookWith(page, slug)).status).toBe(201);

  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect(page).toHaveURL(`/${slug}`);
  await expectOnboardingSteps(db, userId, ["profile", "workspace", "notebook"]);
  await expectNewWorkspace(db, created, userId);
});

test("W11 (page): an account that joined by an invitation goes on past the step, into the workspace", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const email = emailFor(testInfo, "invited");
  const tokens = await register(api, email);
  await accept(api, tokens.access_token, await invite(api, pat, workspace.slug, email, "member"));
  const userId = await accountIdOf(db, email);
  const page = await signedInPage(tokens);
  await page.goto("/");

  // The workspace step records itself as it shows; past the notebook step, the account lands in its workspace.
  const recorded = stepRecorded(page, "workspace");
  expect(await saveProfileStep(page)).toBe(200);
  expect((await recorded).status()).toBe(200);
  await expect(notebookStep(page)).toBeVisible();
  expect((await createFirstNotebookWith(page, workspace.slug)).status).toBe(201);
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect(page).toHaveURL(`/${workspace.slug}`);
  await expectOnboardingSteps(db, userId, ["profile", "workspace", "notebook"]);
  expect(await membershipsOf(db, userId)).toBe(1);
});

test("W11 (page): with creation off, an account without a workspace reads how to get into one, and goes on", async ({
  db,
  nervewikiWith,
  signedInPage,
}, testInfo) => {
  const { baseURL } = await nervewikiWith(db.url, { env: { NWIKI_WORKSPACE__CREATION_ENABLED: "false" } });
  const email = emailFor(testInfo);
  const page = await signedInPage(await register(createClient({ baseUrl: baseURL }), email), baseURL);
  const userId = await accountIdOf(db, email);
  await page.goto(`${baseURL}/`);
  expect(await saveProfileStep(page)).toBe(200);

  await expect(workspaceStep(page)).toBeVisible();
  await expect(page.getByText(/workspaces are created by its administrator/)).toBeVisible();
  await expect(nameField(page)).toHaveCount(0);
  const recorded = stepRecorded(page, "workspace");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  expect((await recorded).status()).toBe(200);
  // Without a workspace, the notebook step only says where notebooks are created.
  await expect(notebookStep(page)).toBeVisible();
  await expect(
    page.getByText(
      "Notebooks are created in a workspace where you are a member, not a guest. Once you are one, use New notebook in its left column.",
      { exact: true }
    )
  ).toBeVisible();
  await page.getByRole("button", { name: "Continue", exact: true }).click();

  await expectCreatePage(page, baseURL);
  await expectOnboardingSteps(db, userId, ["profile", "workspace", "notebook"]);
  expect(await membershipsOf(db, userId)).toBe(0);
});
