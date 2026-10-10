import { useId } from "react";
import useSWR from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { useT, type PlainKey, type Translate } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TransferFailure, TransferJob, TransferProblem } from "../../services/transfer.service";
import { useTransfers } from "../../stores/context";

/** The text of each failure's code (M7/P5 design 4.4, P6 design 3.8). */
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
  tree_changed: "transfer.failure.tree_changed",
};

/** failureText is why a job failed, by its code: one the page does not know is a failure all the same. */
export function failureText(failure: string, t: Translate): string {
  const key = (failures as Partial<Record<string, PlainKey>>)[failure];
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

/** problemText says what befell a node of the vault, by its code: one the page does not know differs all the same. */
function problemText(problem: TransferProblem, t: Translate): string {
  const text = (problems as Partial<Record<string, (problem: TransferProblem, t: Translate) => string>>)[problem.code];
  return text === undefined ? t("transfer.problem.other", { path: problem.path }) : text(problem, t);
}

/**
 * TransferReport is what an ended job did (M7/P5 design 4.4): its counts,
 * and where the vault differs from the notebook, read as it shows (the
 * list holds no problems). Its failure the row tells.
 */
export function TransferReport({ notebook, job, id }: { notebook: Notebook; job: TransferJob; id: string }) {
  const transfers = useTransfers(notebook);
  const t = useT();
  const titleId = useId();
  const { data, error, mutate } = useSWR(["transfer-job", job.id], () => transfers.detail(job.id));
  const counts = job.report?.counts;
  return (
    <div id={id} className="space-y-3 rounded-md bg-muted/50 p-3 text-sm">
      {counts !== undefined && (
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
          {(
            [
              ["transfer.count.pages", counts.pages],
              ["transfer.count.attachments", counts.attachments],
              ["transfer.count.renamed", counts.renamed],
              ["transfer.count.missing", counts.missing],
            ] as const
          ).map(([label, count]) => (
            <div key={label} className="contents">
              <dt className="text-muted-foreground">{t(label)}</dt>
              <dd>{count}</dd>
            </div>
          ))}
        </dl>
      )}
      {data === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        data.problems.length > 0 && (
          <section className="space-y-1">
            <h3 id={titleId} className="font-medium">
              {t("transfer.problemsTitle")}
            </h3>
            <ul aria-labelledby={titleId} className="list-disc space-y-1 pl-5 break-words">
              {data.problems.map((problem, at) => (
                <li key={`${problem.path} ${at.toString()}`}>{problemText(problem, t)}</li>
              ))}
            </ul>
            {data.problems_truncated && <p className="text-muted-foreground">{t("transfer.problemsTruncated")}</p>}
          </section>
        )
      )}
    </div>
  );
}
