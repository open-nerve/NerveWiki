import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG6, a page's HTML kept safe (M4 design 3; M4/P3 design 3.6): what the
// content's own HTML could run, load or send elsewhere does not reach the
// reading view, and a tag left open stays in its block. In the browser
// (M4/P5 design 3.12) nothing of it runs, breaks the CSP or loads from
// elsewhere: an image is a link to it.

test("PG6 (API): a script, event attributes, styles, script and other hosts' links and a raw image do not reach the reading view; an inline tag left open does not leave its block", async ({
  api,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const content = [
    "<script>alert(1)</script>",
    "",
    'a <span onclick="alert(1)" style="color: red">b</span> c',
    "",
    '[x](javascript:alert(1)) [y](//evil.example/a) <a href="javascript:alert(1)">z</a>',
    "",
    '<img src="https://evil.example/i.png" onerror="alert(1)">',
    "",
    "<em>open",
    "",
    "next",
    "",
  ].join("\n");
  const page = await createPage(api, pat, notebook.id, "Unsafe", null, content);

  const { data, error, response } = await getView(api, pat, page.id);
  expect(response.status, JSON.stringify(error)).toBe(200);
  const html = data?.html ?? "";
  for (const unsafe of ["<script", "alert", "onclick", "onerror", "style=", "javascript:", "evil.example", "<img"]) {
    expect(html).not.toContain(unsafe);
  }
  expect(html).toContain("<p>next</p>");
  expect(html.split("<em>").length).toBe(html.split("</em>").length);
});

test("PG6 (page): in the browser the page runs no script, breaks no CSP rule and asks nothing of another host, its image a link", async ({
  api,
  baseURL,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const content = [
    "<script>alert(1)</script>",
    "",
    'a <span onclick="alert(1)" style="color: red">b</span> c',
    "",
    '[x](javascript:alert(1)) <a href="javascript:alert(1)">z</a>',
    "",
    '<img src="https://evil.example/i.png" onerror="alert(1)">',
    "",
    "![logo](https://images.example/logo.png)",
    "",
    "next",
    "",
  ].join("\n");
  const created = await createPage(api, pat, notebook.id, "Unsafe", null, content);
  const page = await signedInPage(tokens);
  const dialogs: string[] = [];
  page.on("dialog", (dialog) => {
    dialogs.push(dialog.message());
    void dialog.dismiss();
  });
  const elsewhere: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== new URL(baseURL ?? "").origin) {
      elsewhere.push(request.url());
    }
  });

  await page.goto(wikiPagePath(workspace.slug, notebook.id, created.id));
  await expect(pageHeading(page, "Unsafe")).toBeVisible();
  const article = page.getByRole("article");
  await expect(article.getByText("next", { exact: true })).toBeVisible();
  await expect(
    article.locator(".nw-image").getByRole("link", { name: "https://images.example/logo.png" })
  ).toBeVisible();
  await expect(article.locator('script, img, [onclick], [onerror], [style], [href*="javascript:"]')).toHaveCount(0);

  // A click on what was the span runs nothing either.
  await article.locator("p", { hasText: "a b c" }).click();
  expect(dialogs).toEqual([]);
  expect(elsewhere).toEqual([]);
});
