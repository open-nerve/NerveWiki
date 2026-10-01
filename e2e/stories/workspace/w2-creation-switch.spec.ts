import { createClient } from "@nervewiki/api-client";

import { nervewikiWorkspaces, nervewikiWorkspacesFails } from "../../fixtures/admin";
import { accountIdOf } from "../../fixtures/assert/identity";
import { countWorkspaces, expectNewWorkspace, expectNoWorkspaceAdded } from "../../fixtures/assert/workspace";
import { bearer, createToken, emailFor, register, registerOnboarded } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";
import { expectCreatePage, nameField, switcherChoices, workspaceHeading } from "../../fixtures/workspace-pages";
import { slugFor } from "../../fixtures/workspaces";

// W2, the switch of workspace creation (M2 design 3), the server
// administrator's command that creates workspaces while it is off (M2/P4
// design 3.3), and the pages while it is (M2/P5 design 3.5).

test("W2 (API): with creation off, the instance says so and creating answers 403", async ({
  db,
  nervewikiWith,
}, testInfo) => {
  const closed = createClient({
    baseUrl: (await nervewikiWith(db.url, { env: { NWIKI_WORKSPACE__CREATION_ENABLED: "false" } })).baseURL,
  });
  const info = await closed.GET("/api/v0/instance");
  expect(info.data?.workspace_creation_enabled).toBe(false);

  const session = await register(closed, emailFor(testInfo));
  const pat = (await createToken(closed, session.access_token, { name: "W2" })).token;
  const before = await countWorkspaces(db);
  // The switch answers first, even to values that break the rules.
  const refusals = await Promise.all(
    [
      { name: "Acme", slug: slugFor(testInfo) },
      { name: "", slug: "Not A Slug" },
    ].map((body) => closed.POST("/api/v0/workspaces", { body, headers: bearer(pat) }))
  );
  for (const refused of refusals) {
    expect(refused.response.status).toBe(403);
    expect(refused.error?.code).toBe("workspace.creation_disabled");
  }
  await expectNoWorkspaceAdded(db, before);
});

test("W2 (command line): workspaces create makes an account the admin of a new workspace while creation is off; a missing or deactivated account, or a taken slug, changes nothing", async ({
  api,
  db,
}, testInfo) => {
  const closed = { NWIKI_WORKSPACE__CREATION_ENABLED: "false" };
  const email = emailFor(testInfo);
  const pat = (await createToken(api, (await register(api, email)).access_token, { name: "W2" })).token;
  const adminId = await accountIdOf(db, email);
  const slug = slugFor(testInfo);

  const out = await nervewikiWorkspaces(
    db,
    ["create", "--slug", slug, "--name", "Acme", "--admin", email.toUpperCase()],
    closed
  );

  const { data } = await api.GET("/api/v0/workspaces", { headers: bearer(pat) });
  const created = data?.data[0];
  expect(data?.data).toHaveLength(1);
  expect(created).toMatchObject({ slug, name: "Acme", role: "admin" });
  expect(out).toBe(`created workspace ${slug} (${created?.id}) with admin ${email}\n`);
  if (created) {
    await expectNewWorkspace(db, created, adminId);
  }

  const deactivatedEmail = emailFor(testInfo, "deactivated");
  const deactivated = await register(api, deactivatedEmail);
  expect((await api.POST("/api/v0/me/deactivate", { headers: bearer(deactivated.access_token) })).response.status).toBe(
    204
  );
  const before = await countWorkspaces(db);
  const other = slugFor(testInfo, "other");
  await Promise.all(
    (
      [
        [["--slug", other, "--name", "Other", "--admin", emailFor(testInfo, "nobody")], "The account does not exist."],
        [["--slug", other, "--name", "Other", "--admin", deactivatedEmail], "This account is deactivated."],
        [["--slug", slug, "--name", "Other", "--admin", email], "The slug is taken."],
      ] as const
    ).map(([args, message]) => nervewikiWorkspacesFails(db, ["create", ...args], message, closed))
  );
  await expectNoWorkspaceAdded(db, before);
});

test("W2 (page): with creation off, an account without a workspace reads how to get into one, and the switcher offers no creation", async ({
  db,
  nervewikiWith,
  signedInPage,
}, testInfo) => {
  const closed = { NWIKI_WORKSPACE__CREATION_ENABLED: "false" };
  const { baseURL } = await nervewikiWith(db.url, { env: closed });
  const email = emailFor(testInfo);
  const page = await signedInPage(await registerOnboarded(createClient({ baseUrl: baseURL }), email), baseURL);

  await page.goto(`${baseURL}/`);
  await expectCreatePage(page, baseURL);
  await expect(page.getByText(/workspaces are created by its administrator/)).toBeVisible();
  await expect(nameField(page)).toHaveCount(0);

  // The server's administrator creates one for the account: the switcher lists it, and offers no creation.
  const slug = slugFor(testInfo);
  await nervewikiWorkspaces(db, ["create", "--slug", slug, "--name", "Acme", "--admin", email], closed);
  await page.goto(`${baseURL}/`);
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  expect(await switcherChoices(page, "Acme")).toEqual({ workspaces: ["Acme"], others: [] });
});
