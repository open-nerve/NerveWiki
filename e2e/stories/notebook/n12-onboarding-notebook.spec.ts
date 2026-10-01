import { accountIdOf, expectOnboardingSteps } from "../../fixtures/assert/identity";
import { countNotebooks, expectNewNotebook } from "../../fixtures/assert/notebook";
import { bearer, createToken, emailFor, register } from "../../fixtures/auth";
import { accept, invite } from "../../fixtures/invitations";
import { notebookGroups } from "../../fixtures/notebook-pages";
import { createNotebook, listNotebooks } from "../../fixtures/notebooks";
import {
  createFirstNotebookWith,
  notebookStep,
  saveProfileStep,
  stepRecorded,
  workspaceStep,
} from "../../fixtures/onboarding-pages";
import { expect, test } from "../../fixtures/test";
import { createWorkspaceWith, workspaceHeading } from "../../fixtures/workspace-pages";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// N12, onboarding's notebook step (M3 design 3; M3/P4 design 3.6): a new
// account creates its first notebook, private; one that sees a notebook
// already goes on; one that may create none reads where they are created.

test("N12 (API): a token creates the first notebook and records the notebook step", async ({ api, db }, testInfo) => {
  const email = emailFor(testInfo);
  const pat = (await createToken(api, (await register(api, email)).access_token, { name: "N12" })).token;
  const userId = await accountIdOf(db, email);
  const record = (step: string) => api.POST("/api/v0/me/onboarding-steps", { body: { step }, headers: bearer(pat) });
  expect((await record("profile")).response.status).toBe(200);
  expect((await record("workspace")).response.status).toBe(200);
  const workspace = await createWorkspace(api, pat, "Acme", slugFor(testInfo));

  const created = await createNotebook(api, pat, workspace.slug, "My notes");
  const { response, data } = await record("notebook");

  expect(response.status).toBe(200);
  expect(data?.onboarding_steps).toEqual(["profile", "workspace", "notebook"]);
  await expectOnboardingSteps(db, userId, ["profile", "workspace", "notebook"]);
  await expectNewNotebook(db, created, userId);
  expect(created).toMatchObject({ name: "My notes", workspace_access: "none", member_count: 1 });
});

test("N12 (page): a new account creates its workspace, then its first notebook, My notes, and lands with it its own", async ({
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
  const slug = slugFor(testInfo);
  expect((await createWorkspaceWith(page, { name: "Acme", slug, button: "Create and continue" })).status).toBe(201);

  await expect(notebookStep(page)).toBeVisible();
  await expect(page.getByText("Step 3 of 3", { exact: true })).toBeVisible();
  await expect(page.getByLabel("Name", { exact: true })).toHaveValue("My notes");
  const recorded = stepRecorded(page, "notebook");
  const { status, created } = await createFirstNotebookWith(page, slug);
  expect(status).toBe(201);
  expect((await recorded).status()).toBe(200);

  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect(page).toHaveURL(`/${slug}`);
  await expect.poll(() => notebookGroups(page, "Acme")).toEqual({ "My notebooks": ["My notes"] });
  await expectNewNotebook(db, created, userId);
  await expectOnboardingSteps(db, userId, ["profile", "workspace", "notebook"]);
});

test("N12 (page): an account that sees a notebook already goes on past the step, creating none", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  await createNotebook(api, pat, workspace.slug, "Handbook", "viewer");
  const email = emailFor(testInfo, "invited");
  const tokens = await register(api, email);
  await accept(api, tokens.access_token, await invite(api, pat, workspace.slug, email, "member"));
  const userId = await accountIdOf(db, email);
  const page = await signedInPage(tokens);
  await page.goto("/");
  const before = await countNotebooks(db);

  const recorded = stepRecorded(page, "notebook");
  expect(await saveProfileStep(page)).toBe(200);
  expect((await recorded).status()).toBe(200);

  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect.poll(() => notebookGroups(page, "Acme")).toEqual({ "Team notebooks": ["Handbook"] });
  expect(await countNotebooks(db)).toEqual(before);
  await expectOnboardingSteps(db, userId, ["profile", "workspace", "notebook"]);
});

test("N12 (page): a guest everywhere reads where notebooks are created, and goes on", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const email = emailFor(testInfo, "guest");
  const tokens = await register(api, email);
  await accept(api, tokens.access_token, await invite(api, pat, workspace.slug, email, "guest"));
  const userId = await accountIdOf(db, email);
  const page = await signedInPage(tokens);
  await page.goto("/");
  expect(await saveProfileStep(page)).toBe(200);

  await expect(notebookStep(page)).toBeVisible();
  await expect(page.getByText(/^Notebooks are created in a workspace where you are a member/)).toBeVisible();
  await expect(page.getByLabel("Name", { exact: true })).toHaveCount(0);
  const recorded = stepRecorded(page, "notebook");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  expect((await recorded).status()).toBe(200);

  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  expect(await listNotebooks(api, tokens.access_token, workspace.slug)).toEqual([]);
  await expectOnboardingSteps(db, userId, ["profile", "workspace", "notebook"]);
});
