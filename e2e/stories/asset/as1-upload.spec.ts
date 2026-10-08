import { expectUploaded } from "../../fixtures/assert/asset";
import { download, getAsset, listAssets, pngBytes, uploadAsset, utf8 } from "../../fixtures/assets";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// AS1, attachments uploaded, listed, read and downloaded (M7 design 9; M7/P2
// design 3.13): each file streamed to the store, its row beside its node,
// and the same bytes at the addresses the server signed, without a token.
// The page version, the attachments' panel, comes with M7/P4.

test("AS1 (API): files uploaded under a page and at the root are listed, read and downloaded at their signed addresses, the same bytes", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  const notesBytes = utf8("Notes beside the guide.\n");
  const picture = await uploadAsset(
    api,
    pat,
    notebook.id,
    { name: "diagram.png", bytes: pngBytes, type: "image/png" },
    guide.id
  );
  const notes = await uploadAsset(api, pat, notebook.id, { name: "notes.txt", bytes: notesBytes, type: "text/plain" });

  expect([picture.parent_id, picture.mime, picture.width, picture.height]).toEqual([guide.id, "image/png", 2, 3]);
  expect([notes.parent_id, notes.mime, notes.width, notes.height]).toEqual([
    null,
    "application/octet-stream",
    null,
    null,
  ]);
  await expectUploaded(db, nervewiki.storageDir, picture, pngBytes, adminId, "api");
  await expectUploaded(db, nervewiki.storageDir, notes, notesBytes, adminId, "api");

  // The addresses are signed anew by each read: the attachment is the same.
  const unsigned = ({ content_url, download_url, expires_at, ...rest }: typeof picture) => rest;
  expect((await listAssets(api, pat, notebook.id, guide.id)).map(unsigned)).toEqual([unsigned(picture)]);
  expect((await listAssets(api, pat, notebook.id)).map(unsigned)).toEqual([unsigned(notes)]);
  const read = await getAsset(api, pat, picture.id);
  expect(read.response.status).toBe(200);
  expect(read.data && unsigned(read.data)).toEqual(unsigned(picture));

  for (const [asset, bytes] of [
    [picture, pngBytes],
    [notes, notesBytes],
  ] as const) {
    for (const address of [asset.content_url, asset.download_url]) {
      // oxlint-disable-next-line no-await-in-loop -- one download at a time, each checked
      const answer = await download(nervewiki.baseURL, address);
      expect(answer.status, `${asset.name} at ${address}`).toBe(200);
      // oxlint-disable-next-line no-await-in-loop -- its body
      expect(Buffer.compare(Buffer.from(await answer.arrayBuffer()), Buffer.from(bytes))).toBe(0);
    }
  }
});
