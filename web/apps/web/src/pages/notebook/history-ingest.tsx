import { observer } from "mobx-react-lite";
import { Link } from "react-router";

import { useT } from "../../i18n/i18n";
import type { Changeset } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { nodeTarget } from "../node-target";
import { openOf } from "../open-node";
import { Mark } from "../page/page-versions";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNotebook } from "./notebook-layout";

/** Added beside the original kind; ingestion never substitutes for Edit. */
export function IngestMark({ count }: { count: number }) {
  const t = useT();
  return count > 0 ? <Mark>{t("history.ingest")}</Mark> : null;
}

/** API order is meaningful: a null occupies its preview position, without backfilling. */
export const IngestSources = observer(function IngestSources({
  changeset,
  limit,
}: {
  changeset: Pick<Changeset, "ingest_count" | "ingest_sources">;
  limit: 3 | 20;
}) {
  const t = useT();
  const notebook = useNotebook();
  const pages = usePageTree(notebook);
  const { slug } = useWorkspace();
  const { locale } = useStore().preferences;
  if (changeset.ingest_count === 0) return null;
  const count =
    changeset.ingest_count >= 10000 ? "10000+" : new Intl.NumberFormat(locale).format(changeset.ingest_count);
  const home = `/${slug}/notebooks/${notebook.id}`;
  return (
    <div className="space-y-1 text-sm">
      <p className="text-muted-foreground">{t("history.ingestCount", { count })}</p>
      <ul aria-label={t("history.ingestSources")} className="flex flex-wrap gap-x-3 gap-y-1">
        {changeset.ingest_sources.slice(0, limit).map((id, index) => {
          const target = id === null ? undefined : nodeTarget(pages, id);
          const open =
            target === undefined
              ? undefined
              : openOf({ id: target.node.id, kind: target.node.kind, parent: target.node.parent_id }, home);
          return (
            <li key={id ?? `gone-${index.toString()}`} className="max-w-full min-w-0 [overflow-wrap:anywhere]">
              {target !== undefined && open !== undefined ? (
                <Link to={open.to} state={open.state} className="underline underline-offset-4">
                  <bdi>{target.path}</bdi>
                </Link>
              ) : (
                <span className="text-muted-foreground">
                  {id === null
                    ? t("history.sourceDeleted")
                    : pages.nodes === undefined
                      ? "…"
                      : t("history.sourceUnavailable")}
                </span>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
});
