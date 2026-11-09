import { expect, test } from "vitest";

import { translator } from "../../i18n/i18n";
import { jobJSON } from "../../test/jobs-server";
import { jobTitle } from "./transfer-names";

// What a job's row says it does, by its kind (M8/P4 design 4.3): a revert is named after the kind of the change it
// undoes, as the history names it (kindText; an upgrade's rollback's too, M11/P1 S3 review C-N2), one the page does not
// know as it is.

const t = translator("en");

test.each([
  [jobJSON(1, { kind: "export", root_id: null }), "Export of the whole notebook"],
  [jobJSON(2, { kind: "import", name: "Vault.zip" }), "Import of Vault.zip"],
  [jobJSON(3, { kind: "trash_restore", name: "Old notes" }), "Restore of Old notes from the trash"],
  [jobJSON(4, { kind: "revert", name: "import" }), "Undo: Import"],
  [jobJSON(5, { kind: "revert", name: "revert" }), "Undo: Undo change"],
  [jobJSON(6, { kind: "revert", name: "gremlins" }), "Undo: gremlins"],
  [jobJSON(7, { kind: "rollback", name: "upgrade" }), "Undo the upgrade"],
  [jobJSON(8, { kind: "revert", name: "rollback" }), "Undo: Upgrade rollback"],
])("job %#'s title is %s", (job, title) => {
  expect(jobTitle(job, t)).toBe(title);
});

// jobTitle's switch falls to the export's title after it: tsc does not tell a kind left out (M11/P1 review C-N3).
test("an upgrade's rollback is the undo of the upgrade, in Chinese too", () => {
  const job = jobJSON(1, { kind: "rollback", name: "upgrade" });
  expect([jobTitle(job, t), jobTitle(job, translator("zh-CN"))]).toEqual(["Undo the upgrade", "撤销升级"]);
});
