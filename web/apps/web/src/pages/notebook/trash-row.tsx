import { File, ScrollText } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useRef, useState, type Ref } from "react";
import { Link } from "react-router";
import { useSWRConfig } from "swr";

import { arrived } from "../../app/arrival";
import { writesPages } from "../../app/effective-role";
import { useMounted } from "../../app/mounted";
import { errorText } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { formatDateTime, formatRelative } from "../../i18n/format";
import { useT, type Translate } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { TrashItem, TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { AssetIcon, type Located } from "../page/attachment-row";
import { useWorkspace } from "../workspace/workspace-layout";

// A trash item as the trash's list and its preview show it (M8 design 4.5,
// 4.13; M8/P2 design 4.2, 4.3): its row, what is said of it, and its
// restore, which both send.

/**
 * TrashRestored is what the trash's status line says of an item restored:
 * its root as the restore left it, its name numbered where a sibling had
 * it, and the page it went under, null at the notebook's top level.
 */
export type TrashRestored = {
  id: string;
  kind: TreeNode["kind"];
  name: string;
  renamed: boolean;
  parent: { id: string; name: string } | null;
};

/** RestoredState is the route state of the trash arrived at from an item restored in its preview. */
export type RestoredState = { arrived: true; restored: TrashRestored };

/** restoredOf is the item restored the route state says, if it says one. */
export function restoredOf(state: unknown): TrashRestored | undefined {
  if (typeof state !== "object" || state === null || !("restored" in state)) {
    return undefined;
  }
  const restored = state.restored;
  return typeof restored === "object" && restored !== null && "id" in restored && typeof restored.id === "string"
    ? (restored as TrashRestored)
    : undefined;
}

/** fromText says where item was: the pages it was under, from the root down, or the notebook's top level. */
function fromText(item: TrashItem, t: Translate): string {
  return item.ancestors.length === 0
    ? t("trash.fromTop")
    : t("trash.from", { path: item.ancestors.map((ancestor) => ancestor.name).join(" / ") });
}

/**
 * restoreToText says where a restore puts item: under its parent where it
 * was, or at the notebook's top level, where it was or, its parent in the
 * trash or its pages too deep under it, instead.
 */
function restoreToText(item: TrashItem, t: Translate): string {
  const { parent_id: parent, reason } = item.restore_to;
  switch (reason) {
    case "original": {
      const name = item.ancestors.find((ancestor) => ancestor.id === parent)?.name;
      return name === undefined ? t("trash.toTop") : t("trash.toParent", { parent: name });
    }
    case "parent_in_trash":
      return t("trash.toTopParentInTrash");
    case "too_deep":
      return t("trash.toTopTooDeep");
  }
}

/** restoredText says where an item was restored, and under which name. */
export function restoredText(restored: TrashRestored, t: Translate): string {
  const { name, parent } = restored;
  if (parent === null) {
    return restored.renamed ? t("trash.restoredAsTop", { name }) : t("trash.restoredTop", { name });
  }
  return restored.renamed
    ? t("trash.restoredAsUnder", { name, parent: parent.name })
    : t("trash.restoredUnder", { name, parent: parent.name });
}

/**
 * openOf is where Open goes for a node restored, of the notebook whose
 * home is home: a page to itself, arrived at; an attachment to where it
 * is, under parent, its parent page or the notebook's home, its row there
 * taking the focus (Located), as a search's result will (M8 design 4.12).
 */
export function openOf(
  { id, kind, parent }: { id: string; kind: TreeNode["kind"]; parent: string | null },
  home: string
): { to: string; state: typeof arrived | Located } {
  if (kind === "page") {
    return { to: `${home}/pages/${id}`, state: arrived };
  }
  return { to: parent === null ? home : `${home}/pages/${parent}`, state: { attachment: id } };
}

/**
 * refusalText says why item was not restored: changed meanwhile (restored
 * already, deleted again, purged in part), past its time, joined to
 * another item, deleted with a page above it since, or too large to
 * restore here, with its counts; anything else as the app says it.
 */
export function refusalText(error: unknown, item: TrashItem, t: Translate): string | undefined {
  if (error instanceof ApiError) {
    switch (error.code) {
      case "page.not_in_trash":
      case "page.trash_changed":
        return t("trash.changed", { name: item.name });
      case "page.restore_expired":
        return t("trash.expired", { name: item.name });
      case "page.not_trash_root":
        return t("trash.merged", { name: item.name });
      case "page.restore_too_large":
        return t("trash.tooLarge", { name: item.name, pages: item.pages, assets: item.assets });
    }
  }
  return errorText(error, t);
}

/** twinKey is what tells two trash items apart in the list: their names, and the names of where they were. */
function twinKey(item: TrashItem): string {
  return [item.name, ...item.ancestors.map((ancestor) => ancestor.name)].join("\n");
}

/**
 * twinsOf is the ids of the items of list that another of list has the
 * same name and place as: nothing else named tells them apart, so their
 * controls name them by when they were deleted too (M8/P2 design 4.2).
 */
export function twinsOf(list: readonly TrashItem[]): Set<string> {
  const counts = new Map<string, number>();
  for (const item of list) {
    counts.set(twinKey(item), (counts.get(twinKey(item)) ?? 0) + 1);
  }
  return new Set(list.filter((item) => (counts.get(twinKey(item)) ?? 0) > 1).map((item) => item.id));
}

/**
 * namesOf are how the link to item's preview and its Restore are named:
 * by its name and where it was, and a twin's by when it was deleted too.
 */
export function namesOf(item: TrashItem, twin: boolean, t: Translate, locale: Locale) {
  const { name } = item;
  const from = fromText(item, t);
  if (!twin) {
    return { link: t("trash.linkName", { name, from }), restore: t("trash.restoreLabel", { name, from }) };
  }
  const time = formatDateTime(item.deleted_at, locale);
  return {
    link: t("trash.linkNameTwin", { name, from, time }),
    restore: t("trash.restoreLabelTwin", { name, from, time }),
  };
}

/** ItemIcon is the icon of item's kind, for the eye: a page's, or its attachment's type's. */
export function ItemIcon({ item }: { item: TrashItem }) {
  if (item.kind === "page") {
    return <ScrollText aria-hidden className="size-4 shrink-0 text-muted-foreground" />;
  }
  return item.asset === null ? (
    <File aria-hidden className="size-4 shrink-0 text-muted-foreground" />
  ) : (
    <AssetIcon mime={item.asset.mime} />
  );
}

/**
 * TrashFacts is what the trash says of item, line by line: where it was,
 * who deleted it, as long ago before now as it is (the time in full as its
 * title), until when it can be restored, how many pages and attachments
 * it holds, and where a restore puts it.
 */
export function TrashFacts({ item, now }: { item: TrashItem; now: number }) {
  const t = useT();
  const { locale } = useStore().preferences;
  return (
    <>
      <p className="text-sm break-words text-muted-foreground">{fromText(item, t)}</p>
      <p className="text-sm break-words text-muted-foreground">
        {t("trash.deletedBy", { name: item.deleted_by.display_name })}
        {" · "}
        <time dateTime={item.deleted_at} title={formatDateTime(item.deleted_at, locale)}>
          {formatRelative(item.deleted_at, now, locale)}
        </time>
      </p>
      <p className="text-sm text-muted-foreground">
        {t("trash.until", { time: formatDateTime(item.restorable_until, locale) })}
        {" · "}
        {t("trash.counts", { pages: item.pages, assets: item.assets })}
      </p>
      <p className="text-sm break-words text-muted-foreground">{restoreToText(item, t)}</p>
    </>
  );
}

/**
 * useTrashRestore is how the trash's list and its preview restore an item
 * (M8/P2 design 4.2): through the store, which sends one at a time per
 * node, in turn with the tree's writes. It answers the item restored, or
 * rejects with the refusal. Whatever the answer, the list is read again,
 * and the item refused: the event that would have them read may not come,
 * the stream down (M8/P2 design review B-M2). An item restored is read as
 * its preview shows again, not at once: the preview it was restored from
 * leaves for the trash, and would say first that it is gone. A refusal of
 * a role taken away meanwhile, or of a notebook gone or out of reach, has
 * the notebooks read again too, whose roles the pages follow. SWR's mutate
 * rejects only once the login's cache is gone, the page with it: nothing
 * to do then (M8/P1 fix check 3, R3-1).
 */
export function useTrashRestore(notebook: Notebook): (item: TrashItem) => Promise<TrashRestored> {
  const pages = usePageTree(notebook);
  const workspace = useWorkspace();
  const { mutate } = useSWRConfig();
  return async (item) => {
    try {
      const restored = await pages.restore(item.id);
      const parent = restored.placed.parent_id;
      return {
        id: restored.id,
        kind: restored.kind,
        name: restored.name,
        renamed: restored.renamed,
        // The tree, read again after the restore, names the parent as it is now.
        parent:
          parent === null
            ? null
            : {
                id: parent,
                name: pages.byId(parent)?.name ?? item.ancestors.find((each) => each.id === parent)?.name ?? "",
              },
      };
    } catch (refusal) {
      void mutate(["trash-item", notebook.id, item.id]).catch(() => undefined);
      if (refusal instanceof ApiError && (refusal.code === "forbidden" || refusal.status === 404)) {
        void mutate(["notebooks", workspace.id]).catch(() => undefined);
      }
      throw refusal;
    } finally {
      void mutate(["trash", notebook.id]).catch(() => undefined);
    }
  };
}

/**
 * RestoreButton is an item's Restore, named label: busy while its restore
 * is out, pressed again meanwhile it sends nothing (M8/P2 design review
 * B-M3). Gone with the focus, a reader by now, it gives the focus away
 * (left) before it leaves the document.
 */
export function RestoreButton({
  label,
  restore,
  left,
}: {
  label: string;
  /** Restores the item, saying what came of it: it does not reject. */
  restore: () => Promise<void>;
  left: () => void;
}) {
  const t = useT();
  const mounted = useMounted();
  const [sending, setSending] = useState(false);
  const busy = useRef(false);
  const button = useRef<HTMLButtonElement>(null);
  useFocusLeaving(button, left);

  async function send(): Promise<void> {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setSending(true);
    try {
      await restore();
    } finally {
      busy.current = false;
      if (mounted()) {
        setSending(false);
      }
    }
  }

  return (
    <Button
      ref={button}
      variant="outline"
      aria-label={label}
      aria-busy={sending || undefined}
      aria-disabled={sending || undefined}
      onClick={() => void send()}
    >
      {t("trash.restore")}
    </Button>
  );
}

/**
 * TrashRow is an item of the trash's list (M8/P2 design 4.2): its kind's
 * icon, its name, which leads to its preview, and what is said of it
 * (TrashFacts); a writer's Restore. Its link and Restore name it by where
 * it was, and a twin by when it was deleted too. Gone with the focus,
 * restored or read away, it gives the focus away (left) before it leaves.
 */
export const TrashRow = observer(function TrashRow({
  notebook,
  item,
  twin,
  restore,
  left,
  linkRef,
  now,
}: {
  notebook: Notebook;
  item: TrashItem;
  /** The time the list was shown at, which its items' times are said before. */
  now: number;
  /** Whether another item listed has its name and place. */
  twin: boolean;
  restore: () => Promise<void>;
  left: () => void;
  linkRef?: Ref<HTMLAnchorElement>;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const { locale } = useStore().preferences;
  const row = useRef<HTMLLIElement>(null);
  useFocusLeaving(row, left);
  const names = namesOf(item, twin, t, locale);
  return (
    <li ref={row} className="flex flex-wrap items-start justify-between gap-4 p-4">
      <div className="min-w-0 space-y-1">
        <p className="flex items-center gap-2 font-medium">
          <ItemIcon item={item} />
          <Link
            ref={linkRef}
            to={`/${slug}/notebooks/${notebook.id}/trash/${item.id}`}
            aria-label={names.link}
            className="min-w-0 break-words underline underline-offset-4"
          >
            {item.name}
          </Link>
        </p>
        <TrashFacts item={item} now={now} />
      </div>
      {writesPages(notebook.role) && <RestoreButton label={names.restore} restore={restore} left={left} />}
    </li>
  );
});
