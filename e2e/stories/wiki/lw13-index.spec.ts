/* oxlint-disable no-await-in-loop -- Pagination and fixture writes have a deliberate order. */
import type { Route } from "@playwright/test";

import { bearer, createToken, emailFor } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { mcpClient, resultOf } from "../../fixtures/mcp";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

for (const mode of ["standard", "llm_wiki"] as const) {
  test(`LW13: a reader pages and filters the ${mode} directory with complete types`, async ({
    api,
    nervewiki,
  }, info) => {
    const team = await newTeam(api, info);
    const notebook = await createNotebook(api, team.pat, team.workspace.slug, "Directory", "none", {
      mode,
      templateLocale: "en",
    });
    const notes: string[] = [];
    const topics: string[] = [];
    for (const [type, names] of [
      ["note", ["Alpha", "Beta", "Gamma"]],
      ["topic", ["Delta", "Epsilon"]],
    ] as const) {
      for (const name of names) {
        const page = await createPage(
          api,
          team.pat,
          notebook.id,
          name,
          null,
          `---\ntype: ${type}\nsummary: ${name}\ntags: [keep]\n---\n${name}`
        );
        (type === "note" ? notes : topics).push(page.id);
      }
    }
    if (notebook.source_root_id)
      await createPage(
        api,
        team.pat,
        notebook.id,
        "Source",
        notebook.source_root_id,
        "---\ntype: source-only\n---\nsource payload"
      );
    const reader = await joinAs(api, team.pat, team.workspace.slug, emailFor(info, "reader"), "guest");
    const me = await api.GET("/api/v0/me", { headers: bearer(reader) });
    if (!me.data) throw new Error("reader has no user identity");
    await addedNotebookMember(api, team.pat, notebook.id, me.data.id, "reader");
    const expectedTypes = [
      { type: "note", count: 3 },
      { type: "topic", count: 2 },
    ];
    for (const type of ["note", "topic"]) {
      let cursor: string | undefined;
      let ended = false;
      const ids: string[] = [];
      for (let page = 0; page < 5; page++) {
        const { data, response } = await api.GET("/api/v0/notebooks/{notebook_id}/index", {
          params: {
            path: { notebook_id: notebook.id },
            query: { type, tag: "keep", limit: 1, ...(cursor ? { cursor } : {}) },
          },
          headers: bearer(reader),
        });
        expect(response.status).toBe(200);
        if (!data) throw new Error("directory returned no body");
        expect(data.mode).toBe(mode);
        expect(data.types).toEqual(expectedTypes);
        expect(data.types_complete).toBe(true);
        expect(data.unfilterable_type_pages).toBe(0);
        expect(data.source_area).toEqual(mode === "llm_wiki" ? { root_id: notebook.source_root_id, sources: 1 } : null);
        expect(data.data).toHaveLength(1);
        expect(data.data[0]?.type).toBe(type);
        ids.push(data.data[0]!.page_id);
        if (data.next_cursor === null) {
          ended = true;
          break;
        }
        cursor = data.next_cursor;
      }
      expect(ended, "pagination terminates within the fixture's bound").toBe(true);
      expect(ids).toEqual(type === "note" ? notes : topics);
    }
    const pat = (await createToken(api, reader, { name: "LW13 reader" })).token;
    for (const scope of ["notebook", "account"] as const) {
      const client = await mcpClient(nervewiki.baseURL, pat, scope === "notebook" ? { notebook: notebook.id } : {});
      const response = resultOf<{ pages: { path: string }[] }>(
        await client.call("index", {
          ...(scope === "account" ? { notebook: notebook.id } : {}),
          type: "topic",
          tag: "keep",
        }),
        `${scope} reader index`
      );
      expect(response.pages.map((item) => item.path)).toEqual(["Delta.md", "Epsilon.md"]);
      expect(response).not.toHaveProperty("types");
    }
  });
}

// Real ordinary pages, the unchanged default page size (50), and a reader.
// The topic group straddles More: its one heading must survive the appended row.
const byID = (a: { id: string }, b: { id: string }) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);

test("LW13 (page): an ordinary reader opens Directory from Places, reads two groups across More, and filters exact type and tag", async ({
  api,
  signedInPage,
}, info) => {
  const team = await newOnboardedTeam(api, info);
  const notebook = await createNotebook(api, team.pat, team.workspace.slug, "Catalogue", "none");
  const notes: { id: string; name: string }[] = [];
  const topics: { id: string; name: string; keep: boolean }[] = [];
  for (const name of ["Note keep", "Note omit"]) {
    const page = await createPage(
      api,
      team.pat,
      notebook.id,
      name,
      null,
      `---\ntype: note\ntags: [${name === "Note keep" ? "keep" : "omit"}]\n---\n${name}\n`
    );
    notes.push({ id: page.id, name });
  }
  for (let i = 0; i < 49; i++) {
    const name = `Topic ${String(i).padStart(2, "0")}`;
    const keep = i % 2 === 0;
    const page = await createPage(
      api,
      team.pat,
      notebook.id,
      name,
      null,
      `---\ntype: topic\ntags: [${keep ? "keep" : "omit"}]\n---\n${name}\n`
    );
    topics.push({ id: page.id, name, keep });
  }
  const ordered = [...notes.toSorted(byID), ...topics.toSorted(byID)];
  const reader = await joinOnboarded(api, team.pat, team.workspace.slug, emailFor(info, "reader"), "guest");
  const me = await api.GET("/api/v0/me", { headers: bearer(reader.access_token) });
  if (!me.data) throw new Error("the browser reader has no user identity");
  await addedNotebookMember(api, team.pat, notebook.id, me.data.id, "reader");
  const page = await signedInPage(reader);
  const home = `/${team.workspace.slug}/notebooks/${notebook.id}`;
  const indexAPI = `/api/v0/notebooks/${notebook.id}/index`;
  const indexAnswer = () =>
    page.waitForResponse(
      (response) => response.request().method() === "GET" && new URL(response.url()).pathname === indexAPI
    );
  await page.goto(wikiPagePath(team.workspace.slug, notebook.id, notes[0]!.id));
  await expect(pageHeading(page, notes[0]!.name)).toBeVisible();
  // Edit waits for the tree as well as the title: asked sooner, a writer has none either.
  await expect(pageTree(page, "Catalogue").getByRole("link", { name: notes[0]!.name, exact: true })).toBeVisible();
  await expect(page.getByRole("main").getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);
  const directory = page
    .getByRole("navigation", { name: "More of Catalogue", exact: true })
    .getByRole("link", { name: "Directory", exact: true });
  await expect(directory, "Directory is available in an ordinary notebook").toBeVisible();
  const first = indexAnswer();
  await directory.click();
  const firstResponse = await first;
  expect(firstResponse.status()).toBe(200);
  expect(new URL(firstResponse.url()).search).toBe("");
  await expect(page).toHaveURL(`${home}/index`);
  await expect(page).toHaveTitle("Directory · Catalogue · Nerve Wiki");
  const pages = page.getByRole("region", { name: "Directory pages", exact: true });
  const groups = pages.getByRole("heading", { level: 2 });
  await expect(groups, "both nonempty groups before More").toHaveText(["note", "topic"]);
  await expect(pages.getByRole("link")).toHaveText(ordered.slice(0, 50).map((item) => item.name));
  await expect(pages.locator("section").nth(0).getByRole("link")).toHaveCount(2);
  await expect(pages.locator("section").nth(1).getByRole("link")).toHaveCount(48);
  const next = indexAnswer();
  await page.getByRole("button", { name: "More", exact: true }).click();
  const nextResponse = await next;
  expect(nextResponse.status()).toBe(200);
  const nextParams = new URL(nextResponse.url()).searchParams;
  expect([...nextParams.keys()]).toEqual(["cursor"]);
  expect(nextParams.get("cursor")).not.toBe("");
  await expect(pages.getByRole("link"), "all 51 real pages").toHaveText(ordered.map((item) => item.name));
  expect(await pages.getByRole("link").evaluateAll((links) => links.map((link) => link.getAttribute("href")))).toEqual(
    ordered.map((item) => `${home}/pages/${item.id}`)
  );
  await expect(groups, "the appended topic retains one group").toHaveText(["note", "topic"]);
  await expect(page.getByRole("button", { name: "More", exact: true })).toHaveCount(0);
  await expect(page.getByRole("combobox", { name: "Type", exact: true }).getByRole("option")).toHaveText([
    "All types",
    "note (2)",
    "topic (49)",
  ]);

  const typed = indexAnswer();
  await page.getByRole("combobox", { name: "Type", exact: true }).selectOption("topic");
  const typedResponse = await typed;
  expect([...new URL(typedResponse.url()).searchParams]).toEqual([["type", "topic"]]);
  await expect(page).toHaveURL(`${home}/index?type=topic`);
  await expect(groups).toHaveText(["topic"]);
  await expect(pages.getByRole("link")).toHaveText(topics.toSorted(byID).map((item) => item.name));
  await page.getByRole("textbox", { name: "Tag", exact: true }).fill("keep");
  const tagged = indexAnswer();
  await page.getByRole("button", { name: "Apply tag", exact: true }).click();
  const taggedResponse = await tagged;
  expect([...new URL(taggedResponse.url()).searchParams]).toEqual([
    ["type", "topic"],
    ["tag", "keep"],
  ]);
  await expect(page).toHaveURL(`${home}/index?type=topic&tag=keep`);
  await expect(pages.getByRole("link"), "the literal keep subset, without note or omit").toHaveText(
    topics
      .filter((item) => item.keep)
      .toSorted(byID)
      .map((item) => item.name)
  );
  await expect(pages.getByRole("link")).toHaveCount(25);
  const clear = page.getByRole("button", { name: "Clear filters", exact: true });
  const cleared = indexAnswer();
  await clear.focus();
  await page.keyboard.press("Enter");
  expect(new URL((await cleared).url()).search).toBe("");
  await expect(page).toHaveURL(`${home}/index`);
  await expect(groups).toHaveText(["note", "topic"]);
  await expect(pages.getByRole("link")).toHaveText(ordered.slice(0, 50).map((item) => item.name));
  await expect(clear, "nothing left to clear keeps the focus on the button").toHaveAttribute("aria-disabled", "true");
  await expect(clear).toBeFocused();

  // A row far down the list opens its page at the top, not at the list's scroll.
  await pages.getByRole("link").nth(49).click();
  await expect(page).toHaveURL(`${home}/pages/${ordered[49]!.id}`);
  const title = page.getByRole("main").getByRole("heading", { level: 1, name: ordered[49]!.name, exact: true });
  await expect(title).toBeFocused();
  await expect.poll(() => title.evaluate((heading) => heading.getBoundingClientRect().top)).toBeGreaterThanOrEqual(0);
});

// Pages without a type group by parent, so a later read's rows join a group far above the button.
test("LW13 (page): the last More of untyped pages under two parents moves the reader to its first row", async ({
  api,
  signedInPage,
}, info) => {
  const team = await newOnboardedTeam(api, info);
  const notebook = await createNotebook(api, team.pat, team.workspace.slug, "Untyped", "none");
  const parents = [
    await createPage(api, team.pat, notebook.id, "Left", null, "Left\n"),
    await createPage(api, team.pat, notebook.id, "Right", null, "Right\n"),
  ];
  for (let i = 0; i < 52; i++) {
    await createPage(api, team.pat, notebook.id, `Child ${String(i).padStart(2, "0")}`, parents[i % 2]!.id, `${i}\n`);
  }
  const page = await signedInPage(team.tokens);
  await page.goto(`/${team.workspace.slug}/notebooks/${notebook.id}/index`);
  const pages = page.getByRole("region", { name: "Directory pages", exact: true });
  await expect(pages.getByRole("link")).toHaveCount(50);
  const more = page.getByRole("button", { name: "More", exact: true });
  await more.focus();
  await page.keyboard.press("Enter");
  await expect(pages.getByRole("link")).toHaveCount(54);
  await expect(more).toHaveCount(0);
  const focused = () =>
    page.evaluate(() => {
      const at = document.activeElement;
      return at instanceof HTMLAnchorElement ? at.getBoundingClientRect().top : undefined;
    });
  await expect.poll(focused).toBeGreaterThanOrEqual(0);
  expect(await focused()).toBeLessThan(page.viewportSize()!.height);

  // A reader who scrolls while More reads stays where they went, the focus unmoved.
  const held: Route[] = [];
  await page.route(
    (url) => url.searchParams.has("cursor"),
    (route) => void held.push(route)
  );
  await page.reload();
  await expect(pages.getByRole("link")).toHaveCount(50);
  await more.focus();
  const read = page.waitForRequest((request) => new URL(request.url()).searchParams.has("cursor"));
  await page.keyboard.press("Enter");
  await read;
  await page.mouse.move(640, 360);
  await page.mouse.wheel(0, -300);
  const scrolled = () => page.evaluate(() => window.scrollY);
  let there = -1;
  await expect
    .poll(async () => {
      const before = there;
      there = await scrolled();
      return there === before;
    })
    .toBe(true);
  await Promise.all(held.map((route) => route.fallback()));
  await expect(pages.getByRole("link")).toHaveCount(54);
  await expect(more).toHaveCount(0);
  expect(await scrolled()).toBe(there);
  expect(await focused()).toBeUndefined();
});

test("LW13 (page): at 375 wide a long type and tag wrap inside the directory", async ({ api, signedInPage }, info) => {
  const team = await newOnboardedTeam(api, info);
  const notebook = await createNotebook(api, team.pat, team.workspace.slug, "Long values", "none");
  const long = "x".repeat(180);
  await createPage(api, team.pat, notebook.id, "Long", null, `---\ntype: ${long}\ntags: [${long}]\n---\nLong\n`);
  const page = await signedInPage(team.tokens);
  await page.setViewportSize({ width: 375, height: 800 });
  await page.goto(`/${team.workspace.slug}/notebooks/${notebook.id}/index`);
  await expect(page.getByRole("region", { name: "Directory pages", exact: true }).getByRole("link")).toHaveText([
    "Long",
  ]);
  await expect(page.getByText(`#${long}`, { exact: true })).toBeVisible();
  const width = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    content: document.documentElement.scrollWidth,
  }));
  expect(width).toEqual({ viewport: 375, content: 375 });
});
