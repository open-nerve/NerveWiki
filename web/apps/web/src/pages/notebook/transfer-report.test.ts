import { expect, test } from "vitest";

import { translator } from "../../i18n/i18n";
import type { TransferProblem } from "../../services/transfer.service";
import { failureText, problemText } from "./transfer-report";

// What a job's failure and its report's problems say, by the job's kind (M7/P5 design 4.4, P6 design 4.1).

const t = translator("en");

test.each([
  ["import", "forbidden", "The person who started it could no longer edit the notebook."],
  ["import", "root_not_found", "The page imported under no longer exists."],
  ["export", "forbidden", "The person who started it could no longer read the notebook."],
  ["export", "root_not_found", "The page exported no longer exists."],
  ["import", "not_zip", "The file is not a zip archive, or the archive is damaged."],
  ["import", "too_many_entries", "The zip holds more entries than the server takes."],
  ["import", "unpacked_too_large", "The zip unpacks to more than the server takes."],
  ["import", "tree_changed", "A page the import had written was deleted while it ran."],
  ["import", "storage_full", "The server's storage is full."],
  ["import", "gremlins", "The job failed."],
] as const)("a failure of an %s, %s, says what befell it", (kind, failure, says) => {
  expect(failureText(kind, failure, t)).toBe(says);
});

test.each([
  ["import", "renamed", "a:b.md was imported as a_b: links to its old name do not reach it."],
  ["export", "renamed", "a:b.md is written as a_b: links to its old name do not reach it in Obsidian."],
  ["import", "unsafe_path", "a:b.md was skipped: its path leads outside the zip."],
  ["import", "special_file", "a:b.md was skipped: it is a symbolic link or another special file."],
  ["import", "encrypted", "a:b.md was skipped: it is encrypted."],
  ["import", "unsupported_method", "a:b.md was skipped: it is compressed in a way the server does not read."],
  ["import", "too_compressed", "a:b.md was skipped: it unpacks to far more than its size in the zip."],
  ["import", "name_not_utf8", "a:b.md was skipped: its name is not UTF-8."],
  ["import", "invalid_content", "a:b.md was skipped: it is not UTF-8 text, or holds a NUL character."],
  ["import", "too_large", "a:b.md was skipped: it is larger than the server takes."],
  ["import", "too_deep", "a:b.md was skipped: it would sit more than 10 levels deep in the notebook."],
  ["import", "duplicate", "a:b.md was skipped: an earlier entry of the zip has the same path."],
  ["import", "unreadable", "a:b.md was skipped: its data in the zip is damaged."],
  ["import", "gremlins", "a:b.md was not imported as it was."],
  ["export", "gremlins", "a:b.md differs in the vault."],
] as const)("a problem of an %s, %s, says what befell the file", (kind, code, says) => {
  const problem = { path: "a:b.md", code: code as TransferProblem["code"], to: "a_b" };
  expect(problemText(kind, problem, t)).toBe(says);
});
