import { observer } from "mobx-react-lite";

import { formatBytes, formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { LintItem } from "../../services/wiki.service";
import { useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { nodeTarget, useNodeNavigate } from "../node-target";
import { openOf } from "../open-node";
import { useWorkspace } from "../workspace/workspace-layout";

/** A wanted group has no node; every other result names a page or source. */
export function healthNodeId(item: LintItem): string | undefined {
  return item.rule === "wanted" ? undefined : item.rule === "undigested_sources" ? item.node_id : item.page_id;
}

/** A snapshot's finding, with its node's path and navigation taken from the current tree. */
export const HealthItem = observer(function HealthItem({
  item,
  pages,
  readingTree,
}: {
  item: LintItem;
  pages: PageTreeStore;
  readingTree: boolean;
}) {
  const t = useT();
  const { locale } = useStore().preferences;
  const { slug } = useWorkspace();
  const navigate = useNodeNavigate(pages);
  const id = healthNodeId(item);
  const target = id === undefined ? undefined : nodeTarget(pages, id);
  const open =
    target === undefined
      ? undefined
      : openOf(
          { id: target.node.id, kind: target.node.kind, parent: target.node.parent_id },
          `/${slug}/notebooks/${pages.notebookId}`
        );
  return (
    <li className="min-w-0 space-y-1 py-3 text-sm break-words">
      {id !== undefined && (
        <div>
          {target !== undefined && open !== undefined ? (
            <a
              href={open.to}
              className="underline underline-offset-4"
              onClick={(event) => {
                if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
                event.preventDefault();
                navigate(id);
              }}
            >
              {target.path}
            </a>
          ) : (
            <span className="text-muted-foreground">
              {t(readingTree ? "status.loading" : "health.nodeUnavailable")}
            </span>
          )}
        </div>
      )}
      {"target" in item && (
        <>
          <p className="whitespace-pre-wrap break-all">{item.target}</p>
          {item.target_cut && <p className="text-muted-foreground">{t("health.targetCut")}</p>}
        </>
      )}
      {item.rule === "wanted" && (
        <p className="text-muted-foreground">
          {t("health.referringPages", { count: new Intl.NumberFormat(locale).format(item.pages) })}
        </p>
      )}
      {item.rule === "invalid_sources" && item.not_link && (
        <p className="text-muted-foreground">{t("health.notLink")}</p>
      )}
      {item.rule === "undigested_sources" && (
        <div className="flex flex-wrap gap-x-3 gap-y-1 text-muted-foreground">
          <span>{t(item.kind === "page" ? "health.page" : "health.attachment")}</span>
          <span>{formatBytes(item.bytes, locale)}</span>
          <span>
            {t("health.created")} <time dateTime={item.created_at}>{formatDateTime(item.created_at, locale)}</time>
          </span>
        </div>
      )}
    </li>
  );
});
