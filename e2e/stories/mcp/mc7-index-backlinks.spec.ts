import { pngBytes, uploadAsset } from "../../fixtures/assets";
import { getPageProperties, getTag, listBacklinks } from "../../fixtures/links";
import { errorOf, mcpClient, resultOf, type McpClient } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// MC7, an agent reads a notebook's index and a page's backlinks over MCP (M9 design 9; M9/P4 design 4.2): the
// pages by type, by tag and by parent, those with a type first, each as REST's reads of it one by one; a page's
// backlinks by its path, REST's listBacklinks of it, page after page; an attachment's path, which has none. MC7
// writes nothing through MCP: no assertion of the database.

/** A page of index's answer, as the tool's structuredContent has it. */
interface IndexPage {
  path: string;
  type: string | null;
  summary: string | null;
  tags: string[];
  changed_at: string;
  inbound_links: number;
  parent: string | null;
  cut: string[];
}

/** index's answer. */
interface IndexAnswer {
  pages: IndexPage[];
  next_cursor: string | null;
}

/** A page of backlinks' answer. */
interface Backlink {
  path: string;
  links: number;
  contexts: string[];
}

/** backlinks' answer. */
interface BacklinksAnswer {
  pages: Backlink[];
  next_cursor: string | null;
}

/** index's answer of args through agent, which must be no error. */
async function indexOf(agent: McpClient, args: Record<string, unknown>): Promise<IndexAnswer> {
  return resultOf<IndexAnswer>(await agent.call("index", args), `index ${JSON.stringify(args)}`);
}

/** The paths of index's answer of args, which holds every page: no cursor follows. */
async function indexPaths(agent: McpClient, args: Record<string, unknown>): Promise<string[]> {
  const answer = await indexOf(agent, args);
  expect(answer.next_cursor, `index ${JSON.stringify(args)} on one page`).toBeNull();
  return answer.pages.map((p) => p.path);
}

/** paths ordered by the ids ids gives them, as the index orders pages of one type, and those without one. */
function byId(ids: Map<string, string>, ...paths: string[]): string[] {
  const idOf = (path: string) => ids.get(path) ?? path;
  return paths.toSorted((a, b) => (idOf(a) < idOf(b) ? -1 : idOf(a) > idOf(b) ? 1 : 0));
}

/** s's first 256 bytes of UTF-8, cut at a character's boundary, as index cuts a type or a summary. */
function cut256(s: string): string {
  let out = "";
  for (const char of s) {
    if (new TextEncoder().encode(out + char).length > 256) {
      break;
    }
    out += char;
  }
  return out;
}

test("MC7 (MCP): an agent lists the index by type, tag and parent, typed pages first, each page as REST reads it; reads a page's backlinks by its path as REST's, page after page; an attachment has none", async ({
  api,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const summary = `The roadmap of the year ${"计划".repeat(46)}`;
  expect(new TextEncoder().encode(summary).length).toBe(300);
  const projects = await createPage(api, pat, notebook.id, "Projects", null, "---\ntype: area\n---\n");
  const roadmap = await createPage(
    api,
    pat,
    notebook.id,
    "Roadmap",
    projects.id,
    `---\ntype: project\ntags: [plan, plan/q4]\nsummary: ${summary}\n---\n# Roadmap\n`
  );
  const spec = await createPage(api, pat, notebook.id, "Spec", projects.id, "---\ntype: project\ntags: [plan]\n---\n");
  const notes = await createPage(api, pat, notebook.id, "Notes", projects.id, "[[Projects/Roadmap]] and [[Roadmap]]\n");
  const inbox = await createPage(api, pat, notebook.id, "Inbox", null, "see [[Projects/Roadmap]]\n");
  await uploadAsset(api, pat, notebook.id, { name: "diagram.png", bytes: pngBytes }, projects.id);
  const ids = new Map([
    ["Projects.md", projects.id],
    ["Projects/Roadmap.md", roadmap.id],
    ["Projects/Spec.md", spec.id],
    ["Projects/Notes.md", notes.id],
    ["Inbox.md", inbox.id],
  ]);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });

  // 1. By type: the two projects by id, each under Projects; the roadmap's summary cut at a character within 256
  // bytes, its tags, its inbound links the two pages that link to it, Notes' two links one page.
  const projectsOfType = await indexOf(agent, { type: "project" });
  expect(projectsOfType.pages.map((p) => p.path)).toEqual(byId(ids, "Projects/Roadmap.md", "Projects/Spec.md"));
  expect(projectsOfType.pages.map((p) => [p.type, p.parent])).toEqual([
    ["project", "Projects.md"],
    ["project", "Projects.md"],
  ]);
  const roadmapItem = projectsOfType.pages.find((p) => p.path === "Projects/Roadmap.md");
  expect(roadmapItem).toMatchObject({
    summary: cut256(summary),
    cut: ["summary"],
    tags: ["plan", "plan/q4"],
    inbound_links: 2,
  });
  expect(new TextEncoder().encode(roadmapItem?.summary ?? "").length).toBe(255);

  // 2. By tag, and a tag under it; a leading # is dropped.
  const planned = byId(ids, "Projects/Roadmap.md", "Projects/Spec.md");
  expect(await indexPaths(agent, { tag: "plan" })).toEqual(planned);
  expect(await indexPaths(agent, { tag: "#plan" })).toEqual(planned);
  expect(await indexPaths(agent, { tag: "plan/q4" })).toEqual(["Projects/Roadmap.md"]);

  // 3. By parent: a page's child pages, the attachment not among them; the root's; "" refused for parent and type.
  expect(await indexPaths(agent, { parent: "Projects.md" })).toEqual([...planned, "Projects/Notes.md"]);
  expect(await indexPaths(agent, { parent: "/" })).toEqual(["Projects.md", "Inbox.md"]);
  for (const field of ["parent", "type"]) {
    // oxlint-disable-next-line no-await-in-loop -- two refusals, one after the other
    const refused = errorOf(await agent.call("index", { [field]: "" }), `index of ${field} ""`);
    expect([refused.code, (refused.fields as { field: string }[]).map((f) => f.field)]).toEqual([
      "validation_failed",
      [field],
    ]);
  }

  // 4. The whole index: typed pages first, by type, then the others by id; each page as REST reads it one by one, its
  // type and summary getPageProperties' (before the cut), its tags getTag's, its inbound links listBacklinks' pages.
  const all = await indexOf(agent, {});
  expect(all.next_cursor).toBeNull();
  expect(all.pages.map((p) => p.path)).toEqual([
    "Projects.md",
    ...planned,
    ...byId(ids, "Projects/Notes.md", "Inbox.md"),
  ]);
  const tagged = new Map<string, string[]>();
  for (const [tag, answer] of await Promise.all(
    ["plan", "plan/q4"].map(async (name) => [name, await getTag(api, pat, notebook.id, name)] as const)
  )) {
    for (const { id } of answer.data?.data ?? []) {
      tagged.set(id, [...(tagged.get(id) ?? []), tag]);
    }
  }
  const reads = await Promise.all(
    all.pages.map(async (p) => {
      const id = ids.get(p.path) ?? "";
      const [properties, backlinks] = await Promise.all([
        getPageProperties(api, pat, id),
        listBacklinks(api, pat, id, { limit: 100 }),
      ]);
      const value = (key: string) => properties.data?.properties.find((prop) => prop.key === key)?.value ?? null;
      return { p, id, type: value("type"), summary: value("summary"), backlinks: backlinks.data?.data.length };
    })
  );
  for (const { p, id, type, summary: whole, backlinks } of reads) {
    expect(p.type, `${p.path}'s type`).toBe(type);
    expect(p.summary, `${p.path}'s summary`).toBe(whole === null ? null : cut256(String(whole)));
    expect(p.tags.toSorted(), `${p.path}'s tags`).toEqual((tagged.get(id) ?? []).toSorted());
    expect(p.inbound_links, `${p.path}'s inbound links`).toBe(backlinks);
  }

  // 5. A page's backlinks by its path: REST's listBacklinks of it, ids as paths, the same counts and lines; one at a
  // time, a cursor to the other.
  const paths = new Map([...ids].map(([path, id]) => [id, path]));
  const { data: rest } = await listBacklinks(api, pat, roadmap.id);
  const want = (rest?.data ?? []).map((b) => ({ path: paths.get(b.id) ?? b.id, links: b.count, contexts: b.contexts }));
  expect(want.map((b) => [b.path, b.links])).toEqual(
    byId(ids, "Projects/Notes.md", "Inbox.md").map((path) => [path, path === "Inbox.md" ? 1 : 2])
  );
  const backlinks = resultOf<BacklinksAnswer>(
    await agent.call("backlinks", { path: "Projects/Roadmap.md" }),
    "backlinks of Roadmap"
  );
  expect(backlinks).toEqual({ pages: want, next_cursor: null });
  const first = resultOf<BacklinksAnswer>(
    await agent.call("backlinks", { path: "Projects/Roadmap.md", limit: 1 }),
    "backlinks of Roadmap, one"
  );
  expect(first.pages).toEqual(want.slice(0, 1));
  expect(first.next_cursor).not.toBeNull();
  const second = resultOf<BacklinksAnswer>(
    await agent.call("backlinks", { path: "Projects/Roadmap.md", limit: 1, cursor: first.next_cursor }),
    "backlinks of Roadmap, the next"
  );
  expect(second).toEqual({ pages: want.slice(1), next_cursor: null });

  // 6. An attachment's path names no page: page.not_found, in page's tools' words.
  const attachment = errorOf(
    await agent.call("backlinks", { path: "Projects/diagram.png" }),
    "backlinks of the attachment"
  );
  expect(attachment.code).toBe("page.not_found");
  expect(attachment.message).toContain("is an attachment; a page's path ends with .md.");
});
