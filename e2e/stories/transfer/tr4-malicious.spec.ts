import { createClient } from "@nervewiki/api-client";

import { expectImported, sha256Of, storedImports, treeOf } from "../../fixtures/assert/transfer";
import { pngBytes } from "../../fixtures/assets";
import { createNotebook } from "../../fixtures/notebooks";
import { listNodes } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, postImport, startImport } from "../../fixtures/transfer";
import { newTeam } from "../../fixtures/workspaces";
import { zipOf, type ZipFile } from "../../fixtures/zip-write";

// TR4, malicious archives (M7 design 4.11, 9; M7/P6 design 3.18): each
// entry that cannot be imported safely is skipped and reported, the rest
// imported; an archive that is no zip, whose end record lies about its
// entries, whose central directory is too large, or that unpacks to more
// than the instance takes fails whole, writing nothing; a larger archive
// than the instance takes is refused before it is stored. No archive is
// left in the store.

/** The words of the text the unpacked bomb is made of: one that compresses well below the ratio a bomb has. */
const words = ["alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa", "lambda", "mu"];

/** Text of about n bytes, of words drawn by a fixed sequence. */
function text(n: number, seed: number): string {
  let s = seed;
  const out: string[] = [];
  for (let length = 0; length < n;) {
    // xorshift32
    s ^= s << 13;
    s ^= s >>> 17;
    s ^= s << 5;
    s >>>= 0;
    const word = words[s % words.length] ?? "";
    out.push(word);
    length += word.length + 1;
  }
  return out.join(" ");
}

test("TR4 (API): the entries that cannot be imported safely are skipped and reported, the rest imported; archives that cannot be read safely fail whole, writing nothing", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Inbox");
  const deep = "a/b/c/d/e/f/g/h/i/j/k.md";
  const archive = zipOf([
    { name: "ok.md", data: "fine ![[kept.png]]\n" },
    { name: "kept.png", data: pngBytes, method: 0 },
    { name: "../escape.md", data: "out" },
    { name: "/absolute.md", data: "out" },
    { name: "C:\\drive.md", data: "out" },
    { name: "link.md", data: "/etc/passwd", symlink: true },
    { name: "bomb.bin", data: Buffer.alloc(16 << 20) },
    { name: "secret.md", data: "hidden", encrypted: true },
    { name: "bzip.md", data: "BZh9", method: 12 },
    { name: "dup.md", data: "first\n" },
    { name: "dup.md", data: "second\n" },
    { name: Buffer.from([0x66, 0xff, 0x2e, 0x6d, 0x64]), data: "latin" },
    { name: "nul.md", data: "a\u0000b" },
    { name: "crc.md", data: "checked", badCrc: true },
    { name: deep, data: "too deep" },
  ]);
  const job = await endedJob(api, pat, (await startImport(api, pat, notebook.id, archive)).id);
  expect([job.state, job.report]).toEqual([
    "succeeded",
    { failure: null, counts: { pages: 12, attachments: 1, renamed: 0, missing: 0, skipped: 12 } },
  ]);
  expect(job.problems.toSorted((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0))).toEqual([
    { path: "../escape.md", code: "unsafe_path", to: null },
    { path: "/absolute.md", code: "unsafe_path", to: null },
    { path: "C:\\drive.md", code: "unsafe_path", to: null },
    { path: deep, code: "too_deep", to: null },
    { path: "bomb.bin", code: "too_compressed", to: null },
    { path: "bzip.md", code: "unsupported_method", to: null },
    { path: "crc.md", code: "unreadable", to: null },
    { path: "dup.md", code: "duplicate", to: null },
    { path: "f\ufffd.md", code: "name_not_utf8", to: null },
    { path: "link.md", code: "special_file", to: null },
    { path: "nul.md", code: "invalid_content", to: null },
    { path: "secret.md", code: "encrypted", to: null },
  ]);
  const folders = ["a", "a/b", "a/b/c", "a/b/c/d", "a/b/c/d/e", "a/b/c/d/e/f", "a/b/c/d/e/f/g", "a/b/c/d/e/f/g/h"];
  expect(await treeOf(db, nervewiki.storageDir, notebook.id)).toEqual([
    ...[...folders, "a/b/c/d/e/f/g/h/i", "a/b/c/d/e/f/g/h/i/j"].map((path) => ({ path, kind: "page", content: "" })),
    { path: "dup", kind: "page", content: "first\n" },
    { path: "kept.png", kind: "asset", mime: "image/png", sha256: sha256Of(pngBytes) },
    { path: "ok", kind: "page", content: "fine ![[kept.png]]\n" },
  ]);
  await expectImported(db, nervewiki.storageDir, job.id, {
    creatorId: adminId,
    client: "api",
    root: null,
    state: "succeeded",
    failure: null,
    done: 13,
    total: 13,
    counts: { pages: 12, attachments: 1, renamed: 0, skipped: 12 },
  });

  // Each archive that cannot be read safely fails whole, nothing written.
  const failsWhole = async (what: string, bytes: Buffer, failure: string) => {
    const empty = await createNotebook(api, pat, workspace.slug, what);
    const failed = await endedJob(api, pat, (await startImport(api, pat, empty.id, bytes)).id);
    expect([what, failed.state, failed.report?.failure, failed.problems]).toEqual([what, "failed", failure, []]);
    await expectImported(db, nervewiki.storageDir, failed.id, {
      creatorId: adminId,
      client: "api",
      root: null,
      state: "failed",
      failure,
      done: 0,
      total: 0,
      counts: { pages: 0, attachments: 0, renamed: 0, skipped: 0 },
    });
    expect(await treeOf(db, nervewiki.storageDir, empty.id)).toEqual([]);
  };
  await failsWhole("no zip", Buffer.from("not a zip at all"), "not_zip");
  // Its end record counts its 65,537 entries in 16 bits: 1.
  const many: ZipFile[] = Array.from({ length: 65_537 }, (_, i) => ({ name: `e${i.toString()}.md`, method: 0 }));
  await failsWhole("an end record that lies", zipOf(many), "too_many_entries");
  const comment = Buffer.alloc(65_535, 0x20);
  const large: ZipFile[] = Array.from({ length: 1_030 }, (_, i) => ({
    name: `c${i.toString()}.md`,
    method: 0,
    comment,
  }));
  await failsWhole("a central directory of more than 64 MiB", zipOf(large), "too_many_entries");
  expect(storedImports(nervewiki.storageDir)).toEqual([]);
});

test("TR4 (API): an archive larger than transfer.import_max_bytes is refused before it is stored; one that unpacks to more than transfer.import_max_unpacked_bytes fails whole", async ({
  newDatabase,
  nervewikiWith,
}, testInfo) => {
  const database = await newDatabase("migrated");
  const small = await nervewikiWith(database.url, {
    env: {
      NWIKI_ASSET__MAX_BYTES: String(1 << 20),
      NWIKI_TRANSFER__IMPORT_MAX_BYTES: String(1 << 20),
      NWIKI_TRANSFER__IMPORT_MAX_UNPACKED_BYTES: String(2 << 20),
    },
  });
  const api = createClient({ baseUrl: small.baseURL });
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Inbox");

  const { response, error } = await postImport(api, pat, notebook.id, Buffer.alloc((1 << 20) + 1));
  expect([response.status, error?.code]).toEqual([413, "payload_too_large"]);
  expect(storedImports(small.storageDir)).toEqual([]);

  // Three pages of 768 KiB each, a fifth of it packed: within the upload, past the unpacked bytes.
  const archive = zipOf([0, 1, 2].map((i) => ({ name: `p${i.toString()}.md`, data: text(768 << 10, i + 1) })));
  expect(archive.length).toBeLessThan(1 << 20);
  const job = await endedJob(api, pat, (await startImport(api, pat, notebook.id, archive)).id);
  expect([job.state, job.report?.failure]).toEqual(["failed", "unpacked_too_large"]);
  expect(await listNodes(api, pat, notebook.id)).toEqual([]);
  expect(storedImports(small.storageDir)).toEqual([]);
});
