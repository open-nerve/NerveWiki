import { formatDateTime } from "../../i18n/format";
import type { Translate } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import type { Changeset } from "../../services/page.service";
import type { TransferJob } from "../../services/transfer.service";
import { kindText } from "./changeset-text";

/** The changesets' kinds, which a revert's job is named after. */
const changesetKinds: Record<Changeset["kind"], true> = {
  edit: true,
  import: true,
  version_restore: true,
  trash_restore: true,
  revert: true,
};

/**
 * jobTitle is what a job does: an export of the whole notebook or of a
 * page, an import, a restore of a trash item, or an undo of a change of a
 * kind, its name (M8/P4 design 4.3).
 */
export function jobTitle(job: TransferJob, t: Translate): string {
  switch (job.kind) {
    case "import":
      return t("transfer.importOf", { name: job.name });
    case "trash_restore":
      return t("transfer.restoreOf", { name: job.name });
    case "revert":
      return t("transfer.revertOf", {
        kind: Object.hasOwn(changesetKinds, job.name) ? kindText(job.name as Changeset["kind"], t) : job.name,
      });
  }
  return job.root_id === null ? t("transfer.exportOfNotebook") : t("transfer.exportOfPage", { name: job.name });
}

/**
 * jobNames names each of jobs for its row and its controls (v0.1 design
 * 13.2, item 17): what it does, who started it when not the account me,
 * and when; one that another of jobs is named as, with the last six digits
 * of its id too.
 */
export function jobNames(jobs: TransferJob[], t: Translate, locale: Locale, me: string): Map<string, string> {
  const named = jobs.map((job) => {
    const title = jobTitle(job, t);
    const time = formatDateTime(job.created_at, locale);
    const name =
      job.created_by.user_id === me
        ? t("transfer.jobName", { title, time })
        : t("transfer.jobNameBy", { title, person: job.created_by.display_name, time });
    return [job.id, name] as const;
  });
  const counts = new Map<string, number>();
  for (const [, name] of named) {
    counts.set(name, (counts.get(name) ?? 0) + 1);
  }
  return new Map(
    named.map(([id, name]) => [
      id,
      (counts.get(name) ?? 0) > 1 ? t("transfer.nameWithId", { name, id: id.replaceAll("-", "").slice(-6) }) : name,
    ])
  );
}
