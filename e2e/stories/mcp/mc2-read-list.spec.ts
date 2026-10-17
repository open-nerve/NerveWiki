import { createHash } from "node:crypto";

import { download, pngBytes, uploadAsset, utf8, webpBytes, type UploadFile } from "../../fixtures/assets";
import { expectUploaded } from "../../fixtures/assert/asset";
import { mcpClient, postMcp, type McpClient } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, listNodes, moveNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// MC2, an agent reads and lists a notebook over MCP (M9 design 9; M9/P1
// design 4.4): list in the tree's order and in the paths', read's parts
// that join into the content exactly, the attachments as images or links,
// and the words of a path that names nothing.

/** The origin the server writes its addresses with (fixtures/server.ts). */
const publicOrigin = "http://wiki.example";

/** Guide's content: a byte order mark and CRLF, which read gives as they are. */
const guideContent = "\ufeff---\r\ntitle: Guide\r\n---\r\nLine one\r\nLine two\r\n";

/** Long's content: 3,000 lines of 40 bytes, more than an answer holds. */
const longContent = Array.from(
  { length: 3000 },
  (_, i) => `line ${String(i + 1).padStart(5, "0")} `.padEnd(39, "-") + "\n"
).join("");

/** Wide's content: a line longer than an answer holds, then another. */
const wideContent = "x".repeat(100_000) + "\nend\n";

/** The attachments uploaded under Guide. */
const files: UploadFile[] = [
  { name: "diagram.png", bytes: pngBytes },
  { name: "dot.webp", bytes: webpBytes },
  { name: "spec.pdf", bytes: utf8("%PDF-1.4\n%%EOF\n") },
  {
    name: "logo.svg",
    bytes: utf8(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10"/></svg>`),
  },
];

/** A list's answer, as the tool's structuredContent has it. */
interface Listing {
  items: {
    path: string;
    kind: string;
    has_children: boolean;
    version?: string;
    mime?: string;
    byte_size: number;
    updated_at?: string;
  }[];
  next_cursor: string | null;
  hint?: string;
}

/** list's answer of args through agent, which must be no error. */
async function list(agent: McpClient, args: Record<string, unknown> = {}): Promise<Listing> {
  const answer = await agent.call("list", args);
  expect(answer.isError, `list ${JSON.stringify(args)}: ${answer.text.join(" ")}`).toBe(false);
  return answer.structured as Listing;
}

/** A page's read: its metadata and its slice. */
interface PageRead {
  meta: {
    lines: number;
    total_lines: number;
    next_offset: number | null;
    next_column: number | null;
    [k: string]: unknown;
  };
  text: string;
}

/** read's answer of a page's path through agent, from offset and column. */
async function readPage(agent: McpClient, args: Record<string, unknown>): Promise<PageRead> {
  const answer = await agent.call("read", args);
  expect(answer.isError, `read ${JSON.stringify(args)}: ${answer.text.join(" ")}`).toBe(false);
  expect(answer.structured, "a text tool's answer has no structuredContent").toBeUndefined();
  expect(answer.content.map((c) => c.type)).toEqual(["text", "text"]);
  return { meta: JSON.parse(answer.text[0] ?? "") as PageRead["meta"], text: answer.text[1] ?? "" };
}

/** The parts of path read in turn through agent, from next_offset and next_column, joined; and the first part. */
async function readAll(agent: McpClient, path: string): Promise<{ first: PageRead; joined: string }> {
  const first = await readPage(agent, { path });
  let joined = first.text;
  let part = first;
  for (let i = 0; part.meta.next_offset !== null; i++) {
    expect(i, `the reads of ${path} end`).toBeLessThan(1000);
    // oxlint-disable-next-line no-await-in-loop -- each part goes on from the one before
    part = await readPage(agent, { path, offset: part.meta.next_offset, column: part.meta.next_column ?? 0 });
    joined += part.text;
  }
  return { first, joined };
}

test("MC2: an agent lists a notebook in the tree's order and by glob, reads pages in parts that join into their bytes, and attachments as images or links", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace, adminId } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide", null, guideContent);
  const setup = await createPage(api, pat, notebook.id, "Setup", guide.id);
  await createPage(api, pat, notebook.id, "Alpha", guide.id);
  await createPage(api, pat, notebook.id, "Long", guide.id, longContent);
  const wide = await createPage(api, pat, notebook.id, "Wide", guide.id, wideContent);
  const uploaded = new Map<string, { id: string; bytes: Uint8Array; mime: string }>();
  for (const file of files) {
    // oxlint-disable-next-line no-await-in-loop -- one upload at a time, in the tree's order
    const asset = await uploadAsset(api, pat, notebook.id, file, guide.id);
    // oxlint-disable-next-line no-await-in-loop -- its row
    await expectUploaded(db, nervewiki.storageDir, asset, file.bytes, adminId, "api");
    if (file.name === "dot.webp") {
      expect([asset.width, asset.height], "the upload reads a WebP's sides").toEqual([1, 1]);
    }
    uploaded.set(file.name, { id: asset.id, bytes: file.bytes, mime: asset.mime });
  }
  // Setup's only child is an attachment: Setup has children all the same.
  await uploadAsset(api, pat, notebook.id, { name: "table.csv", bytes: utf8("a,b\n") }, setup.id);
  // The tree's order is neither the creations' (the ids') nor the names'.
  await moveNode(api, pat, wide.id, { parent_id: guide.id, after_id: null });
  const children = (await listNodes(api, pat, notebook.id)).filter((n) => n.parent_id === guide.id);
  expect(children.map((n) => n.name).slice(0, 4)).toEqual(["Wide", "Setup", "Alpha", "Long"]);

  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });

  // 1. list: the root's children, Guide's in the tree's order, globs in the paths' order, pages of a limit.
  expect((await list(agent)).items).toEqual([
    {
      path: "Guide.md",
      kind: "page",
      has_children: true,
      version: `${guide.id}:1`,
      byte_size: Buffer.byteLength(guideContent),
      updated_at: expect.any(String),
    },
  ]);
  const ofGuide = await list(agent, { pattern: "Guide/" });
  expect(ofGuide.items.map((item) => item.path)).toEqual(
    children.map((n) => `Guide/${n.name}${n.kind === "page" ? ".md" : ""}`)
  );
  for (const [item, node] of ofGuide.items.map((it, i) => [it, children[i]] as const)) {
    expect(item.has_children, item.path).toBe(node?.id === setup.id);
    if (node?.kind === "page") {
      expect([item.kind, item.version, typeof item.byte_size]).toEqual(["page", `${node.id}:1`, "number"]);
    } else {
      const upload = uploaded.get(node?.name ?? "");
      expect([item.kind, item.mime, item.byte_size]).toEqual(["attachment", upload?.mime, upload?.bytes.length]);
    }
  }
  expect(ofGuide.next_cursor).toBeNull();
  expect((await list(agent, { pattern: "Guide/Setup/" })).items.map((item) => item.path)).toEqual([
    "Guide/Setup/table.csv",
  ]);
  expect((await list(agent, { pattern: "**/*.md" })).items.map((item) => item.path)).toEqual([
    "Guide.md",
    "Guide/Alpha.md",
    "Guide/Long.md",
    "Guide/Setup.md",
    "Guide/Wide.md",
  ]);
  expect((await list(agent, { pattern: "guide/*.PNG" })).items.map((item) => item.path)).toEqual(["Guide/diagram.png"]);
  const firstTwo = await list(agent, { pattern: "Guide/", limit: 2 });
  expect(firstTwo.items.map((item) => item.path)).toEqual(ofGuide.items.slice(0, 2).map((item) => item.path));
  expect(firstTwo.next_cursor).not.toBeNull();
  const nextTwo = await list(agent, { pattern: "Guide/", limit: 2, cursor: firstTwo.next_cursor });
  expect(nextTwo.items.map((item) => item.path)).toEqual(ofGuide.items.slice(2, 4).map((item) => item.path));

  // 2. read: a page's bytes as they are stored, and a window of its lines.
  const whole = await readPage(agent, { path: "Guide.md" });
  expect([whole.meta.total_lines, whole.meta.next_offset, whole.text]).toEqual([5, null, guideContent]);
  expect((await readPage(agent, { path: "/Guide.md", offset: 2, limit: 1 })).text).toBe("title: Guide\r\n");

  // 3. A page longer than an answer, in parts of lines.
  const long = await readAll(agent, "Guide/Long.md");
  expect(Buffer.byteLength(long.first.text)).toBeLessThanOrEqual(48 * 1024);
  expect(long.first.meta.next_offset).not.toBeNull();
  expect(long.joined === longContent, "Long's parts joined are its content").toBe(true);

  // 4. A line longer than an answer, in parts of the line.
  const wideRead = await readAll(agent, "Guide/Wide.md");
  expect([wideRead.first.meta.lines, wideRead.first.meta.next_offset]).toEqual([1, 1]);
  expect(wideRead.first.meta.next_column).toBeGreaterThan(0);
  expect(wideRead.joined === wideContent, "Wide's parts joined are its content").toBe(true);

  // 5. The attachments: the images inline, the others as links at the public origin.
  for (const [name, mime] of [
    ["diagram.png", "image/png"],
    ["dot.webp", "image/webp"],
  ] as const) {
    // oxlint-disable-next-line no-await-in-loop -- one attachment at a time
    const answer = await agent.call("read", { path: `Guide/${name}` });
    const image = answer.content[1];
    expect(image?.type === "image" && image.mimeType, name).toBe(mime);
    const bytes = uploaded.get(name)?.bytes ?? new Uint8Array();
    expect(image?.type === "image" && Buffer.compare(Buffer.from(image.data, "base64"), Buffer.from(bytes))).toBe(0);
    // oxlint-disable-next-line no-await-in-loop -- its row
    const rows = await db.query(
      `SELECT mime, byte_size::int AS byte_size, encode(sha256, 'hex') AS sha256, width, height FROM asset_blobs
        WHERE node_id = $1`,
      [uploaded.get(name)?.id]
    );
    const meta = JSON.parse(answer.text[0] ?? "") as Record<string, unknown>;
    expect([
      { mime: meta.mime, byte_size: meta.byte_size, sha256: meta.sha256, width: meta.width, height: meta.height },
    ]).toEqual(rows);
    expect(meta.sha256).toBe(createHash("sha256").update(bytes).digest("hex"));
  }
  for (const name of ["spec.pdf", "logo.svg"]) {
    // oxlint-disable-next-line no-await-in-loop -- one attachment at a time
    const answer = await agent.call("read", { path: `Guide/${name}` });
    const link = answer.content[1];
    if (link?.type !== "resource_link") {
      throw new Error(`read ${name} answered ${JSON.stringify(answer.content)}, want a link`);
    }
    expect(link.uri.startsWith(`${publicOrigin}/api/v0/assets/`), link.uri).toBe(true);
    expect([link.mimeType, link.size]).toEqual([uploaded.get(name)?.mime, uploaded.get(name)?.bytes.length]);
    const address = new URL(link.uri);
    // oxlint-disable-next-line no-await-in-loop -- its download, at the server's own address
    const got = await download(nervewiki.baseURL, address.pathname + address.search);
    expect([got.status, got.headers.get("content-disposition")?.startsWith("attachment")]).toEqual([200, true]);
    // oxlint-disable-next-line no-await-in-loop -- its body
    expect(Buffer.compare(Buffer.from(await got.arrayBuffer()), Buffer.from(uploaded.get(name)?.bytes ?? []))).toBe(0);
  }

  // 6. A path that names nothing: an attachment's path where a page is, a glob without wildcards.
  const missing = await agent.call("read", { path: "Guide" });
  expect(missing.isError).toBe(true);
  expect(missing.text[0]?.startsWith("page.not_found"), missing.text[0]).toBe(true);
  expect(missing.text[0]).toContain("Guide.md");
  const unmatched = await list(agent, { pattern: "Guide" });
  expect(unmatched.items).toEqual([]);
  expect(unmatched.hint).toContain("Guide.md");

  // 7. A notebook that is no id is the edge's 404, before the SDK.
  const refused = await postMcp(
    nervewiki.baseURL,
    pat,
    {
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: { protocolVersion: "2025-11-25", capabilities: {}, clientInfo: { name: "e2e-agent", version: "1" } },
    },
    "nope"
  );
  expect([refused.status, refused.headers.get("content-type"), refused.headers.get("cache-control")]).toEqual([
    404,
    "application/problem+json",
    "no-store",
  ]);
  expect(((await refused.json()) as { code: string }).code).toBe("notebook.not_found");
});
