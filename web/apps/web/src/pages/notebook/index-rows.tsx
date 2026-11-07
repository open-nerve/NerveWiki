import { observer } from "mobx-react-lite";
import { Link } from "react-router";

import { arrived } from "../../app/arrival";
import { formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { NotebookIndexItem } from "../../services/linking.service";
import { useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { nodeTarget } from "../node-target";

type Group = { key: string; typeKey: string | null; parent: string | null; items: NotebookIndexItem[] };

/** Server keys identify typed groups. Untyped items use only the current loaded parent's id. */
function groupsOf(items: readonly NotebookIndexItem[]): Group[] {
  const typed: Group[] = [];
  const untyped = new Map<string | null, Group>();
  for (const item of items) {
    if (item.type_key !== null) {
      let group = typed.at(-1);
      if (group === undefined || group.typeKey !== item.type_key) {
        group = { key: `type:${item.type_key}`, typeKey: item.type_key, parent: null, items: [] };
        typed.push(group);
      }
      group.items.push(item);
    } else {
      let group = untyped.get(item.parent_id);
      if (group === undefined) {
        group = { key: `parent:${item.parent_id ?? ""}`, typeKey: null, parent: item.parent_id, items: [] };
        untyped.set(item.parent_id, group);
      }
      group.items.push(item);
    }
  }
  return [...typed, ...untyped.values()];
}

export const IndexRows = observer(function IndexRows({
  items,
  pages,
  home,
  linkRef,
}: {
  items: readonly NotebookIndexItem[];
  pages: PageTreeStore;
  home: string;
  linkRef: (id: string, node: HTMLAnchorElement | null) => void;
}) {
  const t = useT();
  const { locale } = useStore().preferences;
  return (
    <section aria-label={t("index.pages")} className="space-y-6">
      {groupsOf(items).map((group) => {
        const parent =
          group.parent === null
            ? t("index.root")
            : (nodeTarget(pages, group.parent)?.path ??
              (pages.nodes === undefined ? "…" : t("index.parentUnavailable")));
        const title =
          group.typeKey === null
            ? t("index.untyped", { parent })
            : (group.items.find((item) => item.type !== null && item.type.trim() !== "")?.type ?? t("index.typed"));
        return (
          <section key={group.key} className="space-y-2">
            <h2 className="text-lg font-semibold break-words">{title}</h2>
            <ul className="divide-y rounded-md border">
              {group.items.map((item) => (
                <li key={item.page_id} className="space-y-2 p-4">
                  <Link
                    ref={(node) => linkRef(item.page_id, node)}
                    to={`${home}/pages/${item.page_id}`}
                    state={arrived}
                    className="font-medium break-words underline underline-offset-4"
                  >
                    {item.title}
                  </Link>
                  {item.type !== null && item.type.trim() !== "" && (
                    <p className="text-sm text-muted-foreground break-words">{item.type}</p>
                  )}
                  {item.summary !== null && (
                    <p className="break-words">
                      {item.summary}
                      {item.summary_cut && "…"}
                    </p>
                  )}
                  {item.summary_cut && <p className="text-sm text-muted-foreground">{t("index.summaryCut")}</p>}
                  {item.tags.length > 0 && (
                    <p className="flex flex-wrap gap-2 text-sm">
                      {item.tags.map((tag) => (
                        <span key={tag} className="min-w-0 [overflow-wrap:anywhere]">
                          #{tag}
                        </span>
                      ))}
                    </p>
                  )}
                  <p className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted-foreground">
                    <time dateTime={item.updated_at}>{formatDateTime(item.updated_at, locale)}</time>
                    <span>
                      {t(item.inbound_capped ? "index.inboundCapped" : "index.inbound", {
                        count: item.inbound.toLocaleString(locale),
                      })}
                    </span>
                  </p>
                </li>
              ))}
            </ul>
          </section>
        );
      })}
    </section>
  );
});
