import { File, ScrollText } from "lucide-react";
import { Fragment, type Ref } from "react";
import { Link } from "react-router";

import { formatBytes, formatDateTime, formatRelative } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { assetKind } from "../../lib/asset-kind";
import { partsOf } from "../../lib/text-ranges";
import { shortId } from "../../lib/twins";
import type { SearchHit, TextRange } from "../../services/page.service";
import { useStore } from "../../stores/context";
import { openOf } from "../open-node";
import { AssetIcon } from "../page/attachment-row";
import { useWorkspace } from "../workspace/workspace-layout";

/**
 * Twin is how a hit's link tells it from the others listed (v0.1 design
 * 13.2, item 17): by its name alone; by where it is too, another of its
 * name listed elsewhere; or by the end of its id too, another of its name
 * listed in a place of the same names (two notebooks of one name).
 */
export type Twin = "none" | "path" | "id";

/**
 * Marked is text with the hits ranges name marked (M8/P5 design 4.3):
 * highlighted, and bold, not by their colour alone. The page does not
 * find them itself: they are the server's.
 */
function Marked({ text, ranges }: { text: string; ranges: readonly TextRange[] }) {
  let at = 0;
  return partsOf(text, ranges).map(({ text: part, hit }) => {
    const key = at;
    at += part.length;
    return hit ? (
      <mark key={key} className="nw-hit rounded-xs bg-(--nw-mark-background) font-semibold text-inherit">
        {part}
      </mark>
    ) : (
      <Fragment key={key}>{part}</Fragment>
    );
  });
}

/**
 * SearchHitRow is a hit of a search's list (M8/P5 design 4.3): its kind's
 * icon, its name, its hits marked, which leads to it (openOf: a page to
 * itself, an attachment to where it is listed); where it is, from its
 * notebook (named notebook, "Notebook" while the notebooks are not read)
 * down; a page's content around its first hit, its hits marked, cut where
 * the content goes on; when it last changed, before now (in full as its
 * title); an attachment's type and size. The name and the excerpt are
 * written in a bdi: a name of another direction does not turn the row
 * around it. Its link is named by its name, and a twin's by where it is
 * too, and by the end of its id, which the row shows, when that is the
 * same too.
 */
export function SearchHitRow({
  hit,
  notebook,
  twin,
  now,
  linkRef,
}: {
  hit: SearchHit;
  notebook: string | undefined;
  twin: Twin;
  /** The time the list was shown at, which its hits' times are said before. */
  now: number;
  linkRef?: Ref<HTMLAnchorElement>;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const { locale } = useStore().preferences;
  const { node, excerpt, attachment } = hit;
  const home = `/${slug}/notebooks/${node.notebook_id}`;
  const open = openOf({ id: node.id, kind: node.kind, parent: hit.ancestors.at(-1)?.id ?? null }, home);
  const path = pathOf(hit, notebook ?? t("search.notebook"));
  const id = shortId(node.id);
  const label =
    twin === "none"
      ? undefined
      : twin === "path"
        ? t("search.linkTwin", { name: node.name, path })
        : t("search.linkTwinId", { name: node.name, path, id });
  return (
    <li className="space-y-1 p-4">
      <p className="flex items-center gap-2 font-medium">
        {node.kind === "page" ? (
          <ScrollText aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        ) : attachment === null ? (
          <File aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <AssetIcon mime={attachment.mime} />
        )}
        <Link
          ref={linkRef}
          to={open.to}
          state={open.state}
          aria-label={label}
          className="min-w-0 break-words underline underline-offset-4"
        >
          <bdi>
            <Marked text={node.name} ranges={hit.title_hits} />
          </bdi>
        </Link>
      </p>
      <p className="text-sm break-words text-muted-foreground">
        {path}
        {twin === "id" && (
          <>
            {" · "}
            {t("search.id", { id })}
          </>
        )}
      </p>
      {excerpt !== null && (
        <p className="text-sm break-words">
          <bdi>
            {excerpt.cut_start && "…"}
            <Marked text={excerpt.text} ranges={excerpt.hits} />
            {excerpt.cut_end && "…"}
          </bdi>
        </p>
      )}
      <p className="text-sm text-muted-foreground">
        <time dateTime={hit.changed_at} title={formatDateTime(hit.changed_at, locale)}>
          {formatRelative(hit.changed_at, now, locale)}
        </time>
        {attachment !== null && (
          <>
            {" · "}
            {t(`asset.kind.${assetKind(attachment.mime)}`)} · {formatBytes(attachment.byte_size, locale)}
          </>
        )}
      </p>
    </li>
  );
}

/** pathOf is where hit is: its notebook, named notebook, and the pages above it, from the root down. */
export function pathOf(hit: SearchHit, notebook: string): string {
  return [notebook, ...hit.ancestors.map((ancestor) => ancestor.name)].join(" / ");
}
