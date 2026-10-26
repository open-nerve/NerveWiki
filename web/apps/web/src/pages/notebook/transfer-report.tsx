import { observer } from "mobx-react-lite";
import { useId } from "react";
import { Link } from "react-router";
import useSWR from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { problemMessages } from "../../app/problem-messages";
import { useT, type PlainKey, type Translate } from "../../i18n/i18n";
import { shortId, twinsOf } from "../../lib/twins";
import type { Locale } from "../../i18n/locale";
import type { Notebook } from "../../services/notebook.service";
import type { RevertConflict } from "../../services/page.service";
import type { TransferFailure, TransferJob, TransferProblem } from "../../services/transfer.service";
import { useAccount, usePageTree, useStore, useTransfers } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { reasonText } from "./changeset-text";
import { distinctName } from "./distinct-name";

type Report = NonNullable<TransferJob["report"]>;

/** A count a report shows, each with a text of its own. */
type ShownCount = keyof Report["counts"];

/** The text of each failure's code (M7/P5 design 4.4, P6 design 3.8; M8/P4 design 3.9; M9/P3 design 3.25). */
const failures: Record<TransferFailure, PlainKey> = {
  interrupted: "transfer.failure.interrupted",
  timeout: "transfer.failure.timeout",
  forbidden: "transfer.failure.forbidden",
  root_not_found: "transfer.failure.root_not_found",
  storage_full: "transfer.failure.storage_full",
  contributor_conflict: "transfer.failure.contributor_conflict",
  internal: "transfer.failure.internal",
  not_zip: "transfer.failure.not_zip",
  too_many_entries: "transfer.failure.too_many_entries",
  unpacked_too_large: "transfer.failure.unpacked_too_large",
  too_many_nodes: "transfer.failure.too_many_nodes",
  tree_changed: "transfer.failure.tree_changed",
  changeset_reverted: "transfer.failure.changeset_reverted",
  trash_changed: "transfer.failure.trash_changed",
  restore_expired: "transfer.failure.restore_expired",
  revert_conflict: "transfer.failure.revert_conflict",
  changeset_trimmed: "transfer.failure.changeset_trimmed",
  page_locked: "transfer.failure.page_locked",
  refused: "transfer.failure.refused",
  revert_too_large: "transfer.failure.revert_too_large",
};

/**
 * What a kind's failures say otherwise than an export's (M7/P6 design 4.1;
 * M8/P4 design 3.9): an import's, a restore's and a revert's.
 */
const kindFailures: Partial<Record<TransferJob["kind"], Partial<Record<TransferFailure, PlainKey>>>> = {
  import: {
    forbidden: "transfer.importFailure.forbidden",
    root_not_found: "transfer.importFailure.root_not_found",
  },
  trash_restore: {
    forbidden: "transfer.restoreFailure.forbidden",
    tree_changed: "transfer.restoreFailure.tree_changed",
    changeset_reverted: "transfer.restoreFailure.changeset_reverted",
  },
  revert: {
    forbidden: "transfer.revertFailure.forbidden",
    tree_changed: "transfer.revertFailure.tree_changed",
    changeset_reverted: "transfer.revertFailure.changeset_reverted",
  },
};

/**
 * failureText is why a job of kind failed, by its code: one the page does
 * not know is a failure all the same.
 */
export function failureText(kind: TransferJob["kind"], failure: string, t: Translate): string {
  const key =
    (kindFailures[kind] as Partial<Record<string, PlainKey>> | undefined)?.[failure] ??
    (failures as Partial<Record<string, PlainKey>>)[failure];
  return t(key ?? "transfer.failure.unknown");
}

/** The text of each problem's code (M7/P5 design 4.4, P6 design 3.8). */
const problems: Record<TransferProblem["code"], (problem: TransferProblem, t: Translate) => string> = {
  renamed: (problem, t) => t("transfer.problem.renamed", { path: problem.path, to: problem.to ?? "" }),
  file_missing: (problem, t) => t("transfer.problem.file_missing", { path: problem.path }),
  unsafe_path: (problem, t) => t("transfer.problem.unsafe_path", { path: problem.path }),
  special_file: (problem, t) => t("transfer.problem.special_file", { path: problem.path }),
  encrypted: (problem, t) => t("transfer.problem.encrypted", { path: problem.path }),
  unsupported_method: (problem, t) => t("transfer.problem.unsupported_method", { path: problem.path }),
  too_compressed: (problem, t) => t("transfer.problem.too_compressed", { path: problem.path }),
  name_not_utf8: (problem, t) => t("transfer.problem.name_not_utf8", { path: problem.path }),
  invalid_content: (problem, t) => t("transfer.problem.invalid_content", { path: problem.path }),
  too_large: (problem, t) => t("transfer.problem.too_large", { path: problem.path }),
  too_deep: (problem, t) => t("transfer.problem.too_deep", { path: problem.path }),
  duplicate: (problem, t) => t("transfer.problem.duplicate", { path: problem.path }),
  unreadable: (problem, t) => t("transfer.problem.unreadable", { path: problem.path }),
};

/**
 * problemText says what befell a node of the vault, by its code: one the
 * page does not know differs all the same. A node an import renamed is
 * reached in the notebook by its new name.
 */
export function problemText(kind: TransferJob["kind"], problem: TransferProblem, t: Translate): string {
  if (kind === "import" && problem.code === "renamed") {
    return t("transfer.importProblem.renamed", { path: problem.path, to: problem.to ?? "" });
  }
  const text = (problems as Partial<Record<string, (problem: TransferProblem, t: Translate) => string>>)[problem.code];
  if (text !== undefined) {
    return text(problem, t);
  }
  return t(kind === "import" ? "transfer.importProblem.other" : "transfer.problem.other", { path: problem.path });
}

/**
 * The counts a job's report shows, by its kind: an import's files are
 * never missing, an export skips nothing; a restore counts as an import
 * does, a revert what it did to the nodes and the pages (M8/P4 design 4.3).
 */
const shownCounts: Record<string, readonly ShownCount[]> = {
  export: ["pages", "attachments", "renamed", "missing"],
  import: ["pages", "attachments", "renamed", "skipped"],
  trash_restore: ["pages", "attachments", "renamed", "skipped"],
  revert: ["restored", "deleted", "put_back", "written", "skipped"],
};

/** countText is what count of a job of kind is called: a restore's and a revert's skipped, restored by someone first. */
function countText(kind: TransferJob["kind"], count: ShownCount, t: Translate): string {
  return count === "skipped" && (kind === "trash_restore" || kind === "revert")
    ? t("transfer.count.skippedRestored")
    : t(`transfer.count.${count}`);
}

/** numberText writes a count as locale does: "1,501". */
function numberText(count: number, locale: Locale): string {
  return new Intl.NumberFormat(locale).format(count);
}

/**
 * stoppedText is what a restore or a revert stopped on the way leaves,
 * left (written) nodes not come to, and what to do, as its failure has it
 * (M8/P4 design 4.3; P4B review B-M1): what it did stays, the rest in the
 * trash in items of their own; a restore whose part was deleted or moved
 * too deep (tree_changed), the rest alone; whose item went past its time,
 * the rest no longer to restore; undone (changeset_reverted), all of it in
 * the trash again; whose item changed (trash_changed), nothing to add. A
 * revert undoes its undo to go back to before, then the change whole, and
 * completes a group it stopped in the middle of item by item (review
 * A-M4); trimmed, the change can no longer be undone whole; past a batch
 * (revert_too_large), its undo would stop at the same part again, so
 * neither (M9/P3 S3 review C-M3), its group completed item by item still;
 * undone itself (changeset_reverted), nothing to add.
 */
function stoppedText(job: TransferJob, left: string, t: Translate): string[] {
  const failure = job.state === "failed" ? job.report?.failure : undefined;
  if (job.kind === "trash_restore") {
    switch (failure) {
      case "tree_changed":
        return [t("transfer.restoreLeftOnly", { left })];
      case "restore_expired":
        return [t("transfer.restoreLeftExpired", { left })];
      case "changeset_reverted":
        return [t("transfer.restoreLeftUndone")];
      case "trash_changed":
        return [];
    }
    return [t("transfer.restoreLeft", { left })];
  }
  switch (failure) {
    case "changeset_reverted":
      return [];
    case "changeset_trimmed":
      return [t("transfer.revertLeftTrimmed", { left })];
    case "revert_too_large":
      return [t("transfer.revertLeftTrimmed", { left }), t("transfer.revertLeftGroup")];
  }
  return [t("transfer.revertLeft", { left }), t("transfer.revertLeftGroup")];
}

/**
 * TransferReport is what an ended job did (M7/P5 design 4.4, P6 design
 * 4.1): its counts, and where the vault differs from the notebook, or what
 * an import did not take as it was, read as it shows (the list holds no
 * problems). Its failure the row tells. A restore says where it put the
 * item; a revert, that there was nothing to undo; either, stopped, what
 * became of what it did, how many nodes are left, and what to do
 * (stoppedText); a revert, the nodes in its way, read as it shows, or who
 * edits the page it was about to change (M8/P4 design 4.3). Counts are
 * written as the language writes numbers.
 */
export function TransferReport({ notebook, job, id }: { notebook: Notebook; job: TransferJob; id: string }) {
  const transfers = useTransfers(notebook);
  const t = useT();
  const { locale } = useStore().preferences;
  const titleId = useId();
  const { data, error, mutate } = useSWR(["transfer-job", job.id], () => transfers.detail(job.id));
  const report = job.report;
  const counts = report?.counts;
  // A kind this page does not know shows what both kinds count.
  const shown = shownCounts[job.kind] ?? ["pages", "attachments", "renamed"];
  const stopped = job.state === "failed" || job.state === "cancelled";
  // An import that did not succeed leaves what it had written: the units are not undone.
  const kept = job.kind === "import" && stopped && counts !== undefined && (counts.pages > 0 || counts.attachments > 0);
  // A restore or a revert stopped on the way leaves what it did, the rest of its nodes left.
  const { done, total } = job.progress;
  const left =
    (job.kind === "trash_restore" || job.kind === "revert") && stopped && done > 0
      ? stoppedText(job, numberText(total - done, locale), t)
      : [];
  return (
    <div id={id} className="space-y-3 rounded-md bg-muted/50 p-3 text-sm">
      {counts !== undefined && (
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
          {shown.map((count) => (
            <div key={count} className="contents">
              <dt className="text-muted-foreground">{countText(job.kind, count, t)}</dt>
              <dd>{numberText(counts[count], locale)}</dd>
            </div>
          ))}
        </dl>
      )}
      {kept && <p>{t("transfer.importKept")}</p>}
      {job.kind === "trash_restore" && report?.placed != null && <Placed notebook={notebook} placed={report.placed} />}
      {job.kind === "revert" && job.state === "succeeded" && total === 0 && <p>{t("transfer.revertNothing")}</p>}
      {left.map((line) => (
        <p key={line}>{line}</p>
      ))}
      {job.kind === "revert" && report?.lock != null && <Locked notebook={notebook} lock={report.lock} />}
      {report?.refusal != null && <Refused notebook={notebook} refusal={report.refusal} />}
      {data !== undefined && data.conflicts.length > 0 && (
        <Conflicts conflicts={data.conflicts} count={data.conflict_count} />
      )}
      {data === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        data.problems.length > 0 && (
          <section className="space-y-1">
            <h3 id={titleId} className="font-medium">
              {t(job.kind === "import" ? "transfer.importProblemsTitle" : "transfer.problemsTitle")}
            </h3>
            <ul aria-labelledby={titleId} className="list-disc space-y-1 pl-5 break-words">
              {data.problems.map((problem, at) => (
                <li key={`${problem.path} ${at.toString()}`}>{problemText(job.kind, problem, t)}</li>
              ))}
            </ul>
            {data.problems_truncated && <p className="text-muted-foreground">{t("transfer.problemsTruncated")}</p>}
          </section>
        )
      )}
    </div>
  );
}

/**
 * Placed says where a restore put its trash item's root (M8/P4 design
 * 4.3), as the trash says where a restore puts one: back under the page
 * it was under, by its name as the tree has it, or at the notebook's top
 * level, where it was or instead.
 */
const Placed = observer(function Placed({
  notebook,
  placed,
}: {
  notebook: Notebook;
  placed: NonNullable<Report["placed"]>;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { parent_id: parent, reason } = placed;
  switch (reason) {
    case "original": {
      if (parent === null) {
        return <p>{t("transfer.placedTop")}</p>;
      }
      const name = pages.byId(parent)?.name;
      return <p>{name === undefined ? t("transfer.placedOriginal") : t("transfer.placedUnder", { parent: name })}</p>;
    }
    case "parent_in_trash":
      return <p>{t("transfer.placedParentInTrash")}</p>;
    case "too_deep":
      return <p>{t("transfer.placedTooDeep")}</p>;
  }
});

/**
 * Locked says who edits the page a revert stopped at (page_locked), the
 * account itself elsewhere, leading to the page where the tree has it;
 * the page out of the tree, a page of the undo's.
 */
const Locked = observer(function Locked({ notebook, lock }: { notebook: Notebook; lock: NonNullable<Report["lock"]> }) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { me } = useAccount();
  const { slug } = useWorkspace();
  const page = distinctName(pages.tree, notebook, lock.page_id, t);
  if (page === undefined) {
    return (
      <p>
        {lock.user_id === me.id ? t("transfer.lockedSomeSelf") : t("transfer.lockedSome", { name: lock.display_name })}
      </p>
    );
  }
  return (
    <p className="flex flex-wrap gap-x-3">
      <span>
        {lock.user_id === me.id
          ? t("page.lockedTitledSelf", { page })
          : t("page.lockedTitled", { name: lock.display_name, page })}
      </span>
      <Link to={`/${slug}/notebooks/${notebook.id}/pages/${lock.page_id}`} className="underline underline-offset-4">
        {t("transfer.openPage", { page })}
      </Link>
    </p>
  );
});

/** A contract refusal's reason and node, including codes a newer server adds. */
const Refused = observer(function Refused({
  notebook,
  refusal,
}: {
  notebook: Notebook;
  refusal: NonNullable<Report["refusal"]>;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { slug } = useWorkspace();
  const { code, node_id: node } = refusal;
  const page = distinctName(pages.tree, notebook, node, t);
  const reason = Object.hasOwn(problemMessages, code)
    ? t(problemMessages[code as keyof typeof problemMessages])
    : t("problem.other", { code });
  return (
    <>
      <p>{reason}</p>
      {page === undefined ? (
        <p>{t("transfer.refusedSome")}</p>
      ) : (
        <p className="flex flex-wrap gap-x-3">
          <span>{t("transfer.refusedTitled", { name: page })}</span>
          <Link to={`/${slug}/notebooks/${notebook.id}/pages/${node}`} className="underline underline-offset-4">
            {t("transfer.openPage", { page })}
          </Link>
        </p>
      )}
    </>
  );
});

/**
 * Conflicts are the nodes that stopped a revert (revert_conflict), the
 * first hundred of count: each why, by its name, those of one name by the
 * end of their ids too; their links are in the dialog of the change undone,
 * undone again (M8/P4 design 4.3, review B over-design 5).
 */
function Conflicts({ conflicts, count }: { conflicts: readonly RevertConflict[]; count: number }) {
  const t = useT();
  const { locale } = useStore().preferences;
  const titleId = useId();
  const number = (n: number) => numberText(n, locale);
  const twins = twinsOf(
    conflicts.map((conflict) => ({ id: conflict.node_id, name: conflict.name })),
    (node) => node.name
  );
  return (
    <section className="space-y-1">
      <h3 id={titleId} className="font-medium">
        {t("transfer.conflictsTitle", { count: number(count) })}
      </h3>
      {count > conflicts.length && <p>{t("history.conflictsFirst", { count: number(conflicts.length) })}</p>}
      <ul aria-labelledby={titleId} className="list-disc space-y-1 pl-5 break-words">
        {conflicts.map((conflict) => (
          <li key={conflict.node_id}>
            {reasonText(conflict, t)}
            {twins.has(conflict.node_id) && (
              <span className="text-muted-foreground"> {t("history.id", { id: shortId(conflict.node_id) })}</span>
            )}
          </li>
        ))}
      </ul>
      <p className="text-muted-foreground">{t("transfer.conflictsAgain")}</p>
    </section>
  );
}
