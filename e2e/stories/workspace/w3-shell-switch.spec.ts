import { bearer, emailFor, register, registerOnboarded } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { leave } from "../../fixtures/members";
import { expect, test } from "../../fixtures/test";
import { expectCreatePage, switchWorkspace, switcherChoices, workspaceHeading } from "../../fixtures/workspace-pages";
import { createWorkspace, deleteWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// W3, the shell and the switch between workspaces (M2 design 3; M2/P5
// design 3.2, 3.3).

test("W3 (API): the list is the account's workspaces by name; another's slug is not found; one left leaves the list", async ({
  api,
}, testInfo) => {
  const { pat, workspace: acme } = await newTeam(api, testInfo, "Acme");
  const zeta = await createWorkspace(api, pat, "zeta", slugFor(testInfo, "zeta"));
  const beta = await createWorkspace(api, pat, "Beta", slugFor(testInfo, "beta"));

  // By name, whatever its case: Acme, Beta, zeta.
  const { data } = await api.GET("/api/v0/workspaces", { headers: bearer(pat) });
  expect(data?.data.map((w) => w.slug)).toEqual([acme.slug, beta.slug, zeta.slug]);

  // Another account sees none of them.
  const other = (await register(api, emailFor(testInfo, "other"))).access_token;
  const hidden = await api.GET("/api/v0/workspaces/{slug}", {
    params: { path: { slug: acme.slug } },
    headers: bearer(other),
  });
  expect([hidden.response.status, hidden.error?.code]).toEqual([404, "workspace.not_found"]);

  // A member who leaves no longer has it.
  const member = await joinAs(api, pat, beta.slug, emailFor(testInfo, "member"), "member");
  expect((await api.GET("/api/v0/workspaces", { headers: bearer(member) })).data?.data.map((w) => w.slug)).toEqual([
    beta.slug,
  ]);
  await leave(api, member, beta.slug);
  expect((await api.GET("/api/v0/workspaces", { headers: bearer(member) })).data?.data).toEqual([]);
  const gone = await api.GET("/api/v0/workspaces/{slug}", {
    params: { path: { slug: beta.slug } },
    headers: bearer(member),
  });
  expect([gone.response.status, gone.error?.code]).toEqual([404, "workspace.not_found"]);
});

test("W3 (page): / lands on the workspace shown last, else the first by name, else the creation page; the switcher goes between them; another's slug is not found", async ({
  api,
  signedInPage,
}, testInfo) => {
  const tokens = await registerOnboarded(api, emailFor(testInfo));
  const credential = tokens.access_token;
  const acme = await createWorkspace(api, credential, "Acme", slugFor(testInfo, "acme"));
  const beta = await createWorkspace(api, credential, "Beta", slugFor(testInfo, "beta"));
  const zeta = await createWorkspace(api, credential, "zeta", slugFor(testInfo, "zeta"));
  const others = await createWorkspace(
    api,
    (await registerOnboarded(api, emailFor(testInfo, "other"))).access_token,
    "Others",
    slugFor(testInfo, "others")
  );
  const page = await signedInPage(tokens);

  // A device that showed none lands on the first by name.
  await page.goto("/");
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect(page).toHaveURL(`/${acme.slug}`);
  expect(await switcherChoices(page, "Acme")).toEqual({
    workspaces: ["Acme", "Beta", "zeta"],
    others: ["Create workspace"],
  });

  // The switcher goes to another, which the device lands on from then on.
  await switchWorkspace(page, "Acme", "zeta");
  await expect(workspaceHeading(page, "zeta")).toBeVisible();
  await expect(page).toHaveURL(`/${zeta.slug}`);
  expect(await page.evaluate(() => localStorage.getItem("nwiki.workspace"))).toBe(zeta.slug);
  await page.goto("/");
  await expect(workspaceHeading(page, "zeta")).toBeVisible();
  await expect(page).toHaveURL(`/${zeta.slug}`);

  // Once it is gone, / lands on the first again.
  await deleteWorkspace(api, credential, zeta.slug);
  await page.goto("/");
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect(page).toHaveURL(`/${acme.slug}`);

  // Another account's workspace is no page of this one's.
  await page.goto(`/${others.slug}`);
  await expect(page.getByRole("heading", { level: 1, name: "Page not found" })).toBeVisible();

  // Without a workspace, / lands on the creation page.
  await deleteWorkspace(api, credential, acme.slug);
  await deleteWorkspace(api, credential, beta.slug);
  await page.goto("/");
  await expectCreatePage(page);
});
