import { createHash } from "node:crypto";

import { expectContentWritten } from "../../fixtures/assert/page";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { editStatus, saveEdit, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG9, a content's bytes kept (M4 design 3; M4/P4 design 3.12): no line
// break, byte order mark, blank or composition is touched on the way
// through the API, nor by the editor beyond what is typed (M4/P6 design
// 3.3): a line's break stays as written, a new one is the content's main
// one.

const bom = String.fromCodePoint(0xfeff);
const acute = String.fromCodePoint(0x0301);

test("PG9 (API): CRLF, CR alone, mixed line breaks, a byte order mark, trailing blanks and NFD come back byte for byte, their hash the SHA-256 of those bytes", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const contents = [
    "# Title\r\n\r\nLine one\r\nLine two\r\n",
    "a\rb\r\rc\r",
    "a\r\nb\nc\rd",
    `${bom}# Title\n`,
    "line   \nnext\t\t\n   \n",
    `Cafe${acute} and more\n`,
  ];
  await Promise.all(
    contents.map(async (content, i) => {
      const page = await createPage(api, pat, notebook.id, `Page ${i}`);
      const written = await writeContent(api, pat, page.id, { content, base_revision: 1 });
      await expectContentWritten(db, written, content, adminId);
      const read = await readContent(api, pat, page.id);
      expect(Buffer.from(read.content, "utf8").equals(Buffer.from(content, "utf8")), JSON.stringify(read.content)).toBe(
        true
      );
      expect(read.content_hash).toBe(createHash("sha256").update(content, "utf8").digest("hex"));
    })
  );
});

/**
 * edited is raw after the keys typed in the editor below, by the editor's
 * rules: "A" at the start (after the byte order mark), "B" at the first
 * line's end, Enter and "C", "D" at the start of the line after, then
 * Backspace at the start of the next one. The new line's break is the main
 * one, the break written most (the first of a tie); the first line's own
 * break stays at the end of the new line; the break Backspace deletes goes.
 */
function edited(raw: string): string {
  const marked = raw.startsWith(bom);
  const parts = (marked ? raw.slice(1) : raw).split(/(\r\n|\r|\n)/);
  const lines = parts.filter((_, i) => i % 2 === 0);
  const breaks = parts.filter((_, i) => i % 2 === 1);
  const counts = new Map<string, number>();
  for (const written of breaks) {
    counts.set(written, (counts.get(written) ?? 0) + 1);
  }
  const main = [...counts].reduce((most, each) => (each[1] > most[1] ? each : most), ["\n", 0] as [string, number])[0];
  lines[0] = `A${lines[0]}B`;
  lines.splice(1, 0, "C");
  breaks.splice(0, 0, main);
  lines[2] = `D${lines[2]}`;
  lines.splice(2, 2, `${lines[2]}${lines[3]}`);
  breaks.splice(2, 1);
  return (marked ? bom : "") + lines.map((line, i) => line + (breaks[i] ?? "")).join("");
}

for (const [name, content] of [
  ["CRLF", "# Title\r\n\r\nLine one\r\nLine two\r\n"],
  ["CR alone", "a\rb\r\rc\r"],
  ["mixed breaks", "a\r\nb\nc\rd"],
  ["a byte order mark", `${bom}# Title\n\nText\n`],
  // Home stops after a line's leading blanks first: the lines it goes to have none.
  ["trailing blanks", "line   \nnext\t\t\nlast  \n"],
  ["NFD", `Cafe${acute} and more\r\nsecond\r\nthird`],
] as const) {
  test(`PG9 (page): ${name}: typed in the editor at a line's start and end, after the byte order mark, with Enter and Backspace, the content keeps every other byte; a new line's break is the main one`, async ({
    api,
    signedInPage,
  }, testInfo) => {
    const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
    const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
    const created = await createPage(api, pat, notebook.id, "Notes", null, content);
    const page = await signedInPage(tokens);

    await page.goto(wikiPagePath(workspace.slug, notebook.id, created.id));
    await startEditing(page);
    await page.keyboard.press("ControlOrMeta+Home");
    await page.keyboard.type("A");
    await page.keyboard.press("End");
    await page.keyboard.type("B");
    await page.keyboard.press("Enter");
    await page.keyboard.type("C");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Home");
    await page.keyboard.type("D");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Home");
    await page.keyboard.press("Backspace");
    await saveEdit(page);
    const read = await readContent(api, pat, created.id);
    expect(JSON.stringify(read.content)).toBe(JSON.stringify(edited(content)));
  });
}

test("PG9 (page): a composition at a CRLF line's end, by Chromium's input method protocol: Ctrl+S while it composes saves nothing, its end saves the text it ends on, every other byte kept", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const created = await createPage(api, pat, notebook.id, "Notes", null, "第一行\r\n第二行\r\n");
  const page = await signedInPage(tokens);
  const saves: string[] = [];
  page.on("request", (request) => {
    if (request.method() === "PUT" && new URL(request.url()).pathname === `/api/v0/pages/${created.id}/content`) {
      saves.push(request.url());
    }
  });

  await page.goto(wikiPagePath(workspace.slug, notebook.id, created.id));
  await startEditing(page);
  await page.keyboard.press("ControlOrMeta+Home");
  await page.keyboard.press("End");
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Input.imeSetComposition", { text: "ni", selectionStart: 2, selectionEnd: 2 });
  await page.keyboard.press("ControlOrMeta+s");
  // Nothing goes while the composition lasts: a while, then still nothing.
  await page.waitForTimeout(300);
  expect(saves).toEqual([]);
  await cdp.send("Input.insertText", { text: "你" });
  await expect(editStatus(page)).toHaveText("Saved.");
  expect(saves).toHaveLength(1);
  expect((await readContent(api, pat, created.id)).content).toBe("第一行你\r\n第二行\r\n");
});
