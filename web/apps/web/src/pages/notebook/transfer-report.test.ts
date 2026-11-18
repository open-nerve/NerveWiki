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
  [
    "import",
    "too_many_nodes",
    "The zip would make more pages and attachments than one import can, counting the folders in its paths: an import past it could not be undone.",
  ],
  ["import", "tree_changed", "A page the import had written was deleted while it ran."],
  ["import", "changeset_reverted", "What the import had written was undone while it ran."],
  ["import", "storage_full", "The server's storage is full."],
  ["import", "gremlins", "The job failed."],
  [
    "trash_restore",
    "trash_changed",
    "The item in the trash changed before the restore came to it: someone restored it, the purge took it, or it was deleted again.",
  ],
  ["trash_restore", "restore_expired", "The item went past the time it could be restored in."],
  ["trash_restore", "forbidden", "The person who started it could no longer edit the notebook."],
  ["trash_restore", "tree_changed", "While the restore ran, what it had restored was deleted or moved too deep."],
  ["trash_restore", "changeset_reverted", "What the restore had restored was undone while it ran."],
  [
    "trash_restore",
    "interrupted",
    "The job was interrupted: the server stopped or restarted, or the job stopped responding.",
  ],
  ["revert", "revert_conflict", "What the change touched changed since, or could not go back."],
  ["revert", "changeset_trimmed", "The purge took some or all of the change: it can no longer be undone."],
  ["revert", "page_locked", "Someone is editing a page the undo was about to change."],
  ["revert", "refused", "The notebook's rules as an LLM Wiki refused one of its writes."],
  [
    "revert",
    "revert_too_large",
    "Part of the undo had to be done at once and is more than one batch can write: the pages, attachments, history or links it brings back. What was undone before it stays.",
  ],
  ["revert", "forbidden", "The person who started it could no longer edit the notebook."],
  ["revert", "tree_changed", "While the undo ran, the pages it was undoing changed."],
  ["revert", "changeset_reverted", "What the undo had done was itself undone while it ran."],
  ["revert", "gremlins", "The job failed."],
  [
    "rollback",
    "forbidden",
    "The person who started the rollback is no longer an admin of the notebook: another admin can continue it.",
  ],
  [
    "rollback",
    "revert_too_large",
    "The upgrade changed too much, and the rollback stopped: you can end the rollback, keeping the notebook as it is.",
  ],
  ["rollback", "internal", "The server failed, and the rollback stopped: you can continue it."],
  ["rollback", "interrupted", "The rollback was interrupted: you can continue it."],
  ["rollback", "timeout", "The rollback ran past the server's time limit: you can continue it."],
  ["rollback", "gremlins", "The job failed."],
] as const)("a failure of an %s, %s, says what befell it", (kind, failure, says) => {
  expect(failureText(kind, failure, t)).toBe(says);
});

test.each([
  ["forbidden", "开始回滚的人已不是管理员，另一位管理员可以继续。"],
  ["revert_too_large", "这次升级改动太多，回滚停下了：可以结束回滚、保留现在的样子。"],
  ["internal", "服务器出错了，回滚停下了：可以继续回滚。"],
  ["interrupted", "回滚中断了：可以继续回滚。"],
  ["timeout", "回滚超过了服务器的时限：可以继续回滚。"],
] as const)("a rollback that stopped, %s, says what to do next in Chinese too", (failure, says) => {
  expect(failureText("rollback", failure, translator("zh-CN"))).toBe(says);
});

test("a revert the purge took some or all of says so in Chinese too", () => {
  expect(failureText("revert", "changeset_trimmed", translator("zh-CN"))).toBe(
    "这次改动已有部分或全部被清理，不能再撤销。"
  );
});

test("a revert one of whose parts done at once is past a batch says so in Chinese too", () => {
  expect(failureText("revert", "revert_too_large", translator("zh-CN"))).toBe(
    "撤销中必须一次做完的一段太大，一批写不下（它带回的页、附件、历史或链接）。在它之前撤销的部分留着。"
  );
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

// What an undo stopped in the middle of a group says to do, in both languages: restore its items one by
// one; not, since decision 482, "do not undo this undo first" (P3/S3 review A's N2).
test.each([
  [
    "en",
    "If it stopped in the middle of restoring a group, restoring the items it left in the trash one by one completes that group.",
  ],
  ["zh-CN", "停在恢复一组的中间时：回收站里由这次撤销留下的几项，逐项恢复就完成这一组。"],
] as const)("an undo stopped in a group says how to complete it in %s", (language, says) => {
  expect(translator(language)("transfer.revertLeftGroup")).toBe(says);
});

// An import's names read from elsewhere than their headers' bytes (M12/P3 design 4.2): no skip.
test.each([
  [
    "en",
    "name_unicode_path",
    "测试.md: its name was not UTF-8 and was read from the Unicode name the zip tool stored with it.",
  ],
  [
    "en",
    "name_gbk",
    "图片/截图.png: its name was not UTF-8 and was read as GBK, as Chinese Windows writes names. If it reads wrong, rename it.",
  ],
  ["zh-CN", "name_unicode_path", "测试.md：名称不是 UTF-8，取了压缩工具另存的 Unicode 名称。"],
  ["zh-CN", "name_gbk", "图片/截图.png：名称不是 UTF-8，按 GBK（中文 Windows 的写法）读出；读错了的话，给它改个名。"],
] as const)("an import's name read otherwise says how, in %s: %s", (language, code, says) => {
  const path = code === "name_gbk" ? "图片/截图.png" : "测试.md";
  expect(problemText("import", { path, code, to: null }, translator(language))).toBe(says);
});

test.each([
  ["en", "incoming.md was skipped: identical source content already exists at raw/existing.md."],
  ["zh-CN", "已跳过 incoming.md：raw/existing.md 已有字节相同的来源。"],
] as const)("source duplicate import problem names its archive item and target in %s", (language, message) => {
  const problem = { path: "incoming.md", code: "duplicate_source" as TransferProblem["code"], to: "raw/existing.md" };
  expect(problemText("import", problem, translator(language))).toBe(message);
});
