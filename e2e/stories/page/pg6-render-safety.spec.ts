import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG6, a page's HTML kept safe (M4 design 3; M4/P3 design 3.6): what the
// content's own HTML could run, load or send elsewhere does not reach the
// reading view, and a tag left open stays in its block.

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
