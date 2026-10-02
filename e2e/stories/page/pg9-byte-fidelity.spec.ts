import { createHash } from "node:crypto";

import { expectContentWritten } from "../../fixtures/assert/page";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG9, a content's bytes kept (M4 design 3; M4/P4 design 3.12): no line
// break, byte order mark, blank or composition is touched on the way
// through the API.

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
