import { accountIdOf } from "../../fixtures/assert/identity";
import { countWorkspaces, expectNewWorkspace, expectNoWorkspaceAdded } from "../../fixtures/assert/workspace";
import { bearer, createToken, emailFor, register, registerOnboarded } from "../../fixtures/auth";
import { failedToLoad, noteOf } from "../../fixtures/browser";
import { expect, test } from "../../fixtures/test";
import {
  createWorkspaceWith,
  expectCreatePage,
  nameField,
  slugField,
  workspaceHeading,
} from "../../fixtures/workspace-pages";
import { checkSlug, createWorkspace, slugFor } from "../../fixtures/workspaces";

// W1, creating a workspace (M2 design 3).

test("W1 (API): a workspace is created with the caller as its admin, its slug checked first", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);
  const pat = (await createToken(api, session.access_token, { name: "W1" })).token;
  const userId = await accountIdOf(db, email);
  const slug = slugFor(testInfo);

  expect(await checkSlug(api, pat, slug)).toEqual({ available: true });
  expect(await checkSlug(api, pat, "Acme")).toEqual({ available: false, reason: "invalid" });
  expect(await checkSlug(api, pat, "settings")).toEqual({ available: false, reason: "reserved" });

  const created = await createWorkspace(api, pat, "  Acme 研发 ", slug);
  expect(created).toMatchObject({ slug, name: "Acme 研发", role: "admin" });
  await expectNewWorkspace(db, created, userId);
  expect(await checkSlug(api, pat, slug)).toEqual({ available: false, reason: "taken" });

  // The slug taken, a reserved one: nothing is added.
  const before = await countWorkspaces(db);
  const taken = await api.POST("/api/v0/workspaces", { body: { name: "Other", slug }, headers: bearer(pat) });
  expect(taken.response.status).toBe(409);
  expect(taken.error?.code).toBe("workspace.slug_taken");
  const reserved = await api.POST("/api/v0/workspaces", { body: { name: "Other", slug: "api" }, headers: bearer(pat) });
  expect(reserved.response.status).toBe(422);
  expect(reserved.error?.errors).toEqual([{ field: "slug", code: "not_allowed", message: "is reserved" }]);
  await expectNoWorkspaceAdded(db, before);

  // The workspace reads back as it was created, and is the account's one.
  const got = await api.GET("/api/v0/workspaces/{slug}", { params: { path: { slug } }, headers: bearer(pat) });
  expect(got.data).toEqual(created);
  const list = await api.GET("/api/v0/workspaces", { headers: bearer(pat) });
  expect(list.data).toEqual({ data: [created] });

  // Another account does not see it.
  const other = await register(api, emailFor(testInfo, "other"));
  const hidden = await api.GET("/api/v0/workspaces/{slug}", {
    params: { path: { slug } },
    headers: bearer(other.access_token),
  });
  expect(hidden.response.status).toBe(404);
  expect(hidden.error?.code).toBe("workspace.not_found");
});

test("W1 (page): an account without a workspace lands on the creation page, which checks the slug, then goes into the new workspace", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo);
  const tokens = await registerOnboarded(api, email);
  const userId = await accountIdOf(db, email);
  const taken = slugFor(testInfo, "taken");
  await createWorkspace(api, (await registerOnboarded(api, emailFor(testInfo, "other"))).access_token, "Other", taken);
  const page = await signedInPage(tokens);
  await page.goto("/");
  await expectCreatePage(page);

  // The slug follows the name; the server says whether a slug is free, under the field.
  await nameField(page).fill("  Acme 研发 ");
  await expect(slugField(page)).toHaveValue("acme");
  await slugField(page).fill("settings");
  await expect.poll(() => noteOf(slugField(page))).toBe("Reserved by the app. Choose another.");
  await slugField(page).fill(taken);
  await expect.poll(() => noteOf(slugField(page))).toBe("Another workspace already has this address.");

  // Sent all the same, the taken slug is refused under the field, and nothing is added.
  const before = await countWorkspaces(db);
  expect((await createWorkspaceWith(page, { name: "  Acme 研发 ", slug: taken })).status).toBe(409);
  await expect.poll(() => noteOf(slugField(page))).toBe("Another workspace already has this address.");
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  await expectNoWorkspaceAdded(db, before);

  const slug = slugFor(testInfo);
  await slugField(page).fill(slug);
  await expect.poll(() => noteOf(slugField(page))).toBe("Available.");
  const { status, created } = await createWorkspaceWith(page, { name: "  Acme 研发 ", slug });
  expect(status).toBe(201);
  expect(created).toMatchObject({ slug, name: "Acme 研发", role: "admin" });
  await expect(workspaceHeading(page, "Acme 研发")).toBeVisible();
  await expect(page).toHaveURL(`/${slug}`);
  await expectNewWorkspace(db, created, userId);
});
