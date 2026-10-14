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
import { shortId, twinsOf } from "../../lib/twins";
import { ApiError, lost } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { TrashItem, TreeNode } from "../../services/page.service";
import type { TransferJob } from "../../services/transfer.service";
import { useAccount, usePageTree, useStore, useTransfers } from "../../stores/context";
import { AssetIcon, type Located } from "../page/attachment-row";
import { useWorkspace } from "../workspace/workspace-layout";

// A trash item as the trash's list and its preview show it (M8 design 4.5,
// 4.13; M8/P2 design 4.2, 4.3): its row, what is said of it, and its
// restore, which both send; refused as too large to restore here, its
// restore in the background (M8/P4 design 4.2).

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

/**
 * Started is what the trash's status line says of a restore started in
 * the background: the item's name, and its job, whose row the jobs' page
 * gives the focus to (M8/P4 design 4.2).
 */
export type Started = { name: string; job: string };

/** StartedState is the route state of the trash arrived at from an item's restore started in its preview (review B-M5). */
export type StartedState = { arrived: true; started: Started };

/** startedOf is the restore started the route state says, if it says one. */
export function startedOf(state: unknown): Started | undefined {
  if (typeof state !== "object" || state === null || !("started" in state)) {
    return undefined;
  }
  const started = state.started;
  return typeof started === "object" &&
    started !== null &&
    "job" in started &&
    typeof started.job === "string" &&
    "name" in started &&
    typeof started.name === "string"
    ? { name: started.name, job: started.job }
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
function refusalText(error: unknown, item: TrashItem, t: Translate): string | undefined {
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

/**
 * Refusal is a refusal of item's restore: its restore here's, or, start,
 * its start in the background's.
 */
export type Refusal = { error: unknown; item: TrashItem; start?: boolean };

/**
 * refusedText says why refusal's item was not restored, or, refused its
 * start in the background, why that did not start: a job that writes the
 * notebook's tree under way, or too many jobs waiting (M8/P4 design 4.2);
 * else as refusalText.
 */
export function refusedText({ error, item, start }: Refusal, t: Translate): string | undefined {
  if (start === true && error instanceof ApiError) {
    switch (error.code) {
      case "transfer.busy":
        return t("transfer.writingBusy");
      case "server_busy":
        return t("transfer.queueFull");
    }
  }
  return refusalText(error, item, t);
}

/**
 * offeredOf is the item a restore in the background is offered for after
 * refusal (M8/P4 design 4.2): one too large to restore here, or whose start
 * in the background found the notebook or the server busy, to try again;
 * none after any other refusal.
 */
export function offeredOf(refusal: Refusal | undefined): TrashItem | undefined {
  const code = refusal?.error instanceof ApiError ? refusal.error.code : undefined;
  const offered =
    refusal?.start === true ? code === "transfer.busy" || code === "server_busy" : code === "page.restore_too_large";
  return offered ? refusal?.item : undefined;
}

/** twinKey is what tells two trash items apart in the list: their names, and the names of where they were. */
function twinKey(item: TrashItem): string {
  return [item.name, ...item.ancestors.map((ancestor) => ancestor.name)].join("\n");
}

/**
 * Twin is how an item's controls tell it from the others listed: by its
 * name and where it was alone; by when it was deleted too, another of its
 * name and place listed; or by the end of its id too, another of them
 * deleted in the same minute.
 */
export type Twin = "none" | "time" | "id";

/**
 * twinsIn is how each item of list is told apart (M8/P2 design 4.2; v0.1
 * design 13.2, item 17): the items of one name and place (twinsOf) by when
 * each was deleted, as locale writes it, to the minute; those of the same
 * minute too by the last six digits of their ids.
 */
export function twinsIn(list: readonly TrashItem[], locale: Locale): (item: TrashItem) => Twin {
  const twins = twinsOf(list, twinKey);
  const sameMinute = twinsOf(list, (item) => `${twinKey(item)}\n${formatDateTime(item.deleted_at, locale)}`);
  return (item) => (sameMinute.has(item.id) ? "id" : twins.has(item.id) ? "time" : "none");
}

/**
 * namesOf are how the link to item's preview, its Restore and its Restore
 * in the background are named: by its name and where it was, and a twin's
 * by when it was deleted too, and by the end of its id when that is the
 * same to the minute.
 */
export function namesOf(item: TrashItem, twin: Twin, t: Translate, locale: Locale) {
  const { name } = item;
  const from = fromText(item, t);
  if (twin === "none") {
    return {
      link: t("trash.linkName", { name, from }),
      restore: t("trash.restoreLabel", { name, from }),
      background: t("trash.backgroundLabel", { name, from }),
    };
  }
  const time = formatDateTime(item.deleted_at, locale);
  if (twin === "time") {
    return {
      link: t("trash.linkNameTwin", { name, from, time }),
      restore: t("trash.restoreLabelTwin", { name, from, time }),
      background: t("trash.backgroundLabelTwin", { name, from, time }),
    };
  }
  const id = shortId(item.id);
  return {
    link: t("trash.linkNameTwinId", { name, from, time, id }),
    restore: t("trash.restoreLabelTwinId", { name, from, time, id }),
    background: t("trash.backgroundLabelTwinId", { name, from, time, id }),
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
 * title), and the end of its id when a twin's deletion is in the same
 * minute (id); until when it can be restored, how many pages and
 * attachments it holds, and where a restore puts it.
 */
export function TrashFacts({ item, now, id }: { item: TrashItem; now: number; id?: string }) {
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
        {id !== undefined && (
          <>
            {" · "}
            {t("trash.id", { id })}
          </>
        )}
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
 * rejects with the refusal. Whatever the answer, the list and the item
 * are read again: the event that would have them read may not come, the
 * stream down (M8/P2 design review B-M2). The preview the item was
 * restored from keeps what the restore answered over what the item reads
 * meanwhile (trash-preview.tsx). A refusal of a role taken away
 * meanwhile, or of a notebook gone or out of reach, has the notebooks read
 * again too, whose roles the pages follow. SWR's mutate rejects only once
 * the login's cache is gone, the page with it: nothing to do then (M8/P1
 * fix check 3, R3-1).
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
      if (refusal instanceof ApiError && (refusal.code === "forbidden" || refusal.status === 404)) {
        void mutate(["notebooks", workspace.id]).catch(() => undefined);
      }
      throw refusal;
    } finally {
      void mutate(["trash", notebook.id]).catch(() => undefined);
      void mutate(["trash-item", notebook.id, item.id]).catch(() => undefined);
    }
  };
}

/**
 * BackgroundStart is what a restore started in the background came to: its
 * job; or, its answer lost, none of the jobs read again started since it
 * was pressed (none), or the jobs not read either (unread).
 */
export type BackgroundStart = { job: TransferJob } | { lost: "none" | "unread" };

/**
 * useBackgroundRestore is how the trash's list and its preview restore an
 * item in the background, refused as too large to restore here (M8/P4
 * design 4.2): through the jobs' store, a start at a time. It answers the
 * job started, or, its answer lost (a 5xx but busy, no answer), the
 * account's job of the item started since it was pressed among the jobs
 * read again, if any (review B-M6); it rejects with any other refusal.
 * The list and the item are read again, as after a restore; a refusal of
 * a role taken away meanwhile, or of a notebook gone or out of reach, has
 * the notebooks read again too.
 */
export function useBackgroundRestore(notebook: Notebook): (item: TrashItem) => Promise<BackgroundStart> {
  const transfers = useTransfers(notebook);
  const { me } = useAccount();
  const workspace = useWorkspace();
  const { mutate } = useSWRConfig();
  return async (item) => {
    const since = Date.now();
    try {
      return { job: await transfers.startTrashRestore(item.id) };
    } catch (refusal) {
      if (!lost(refusal)) {
        if (refusal instanceof ApiError && (refusal.code === "forbidden" || refusal.status === 404)) {
          void mutate(["notebooks", workspace.id]).catch(() => undefined);
        }
        throw refusal;
      }
    } finally {
      void mutate(["trash", notebook.id]).catch(() => undefined);
      void mutate(["trash-item", notebook.id, item.id]).catch(() => undefined);
    }
    try {
      const job = await transfers.startedJob("trash_restore", item.id, me.id, since);
      return job === undefined ? { lost: "none" } : { job };
    } catch {
      return { lost: "unread" };
    }
  };
}

/**
 * RestoreButton is an item's Restore, named label, or its Restore in the
 * background, which says text: busy while its restore is out, pressed
 * again meanwhile it sends nothing (M8/P2 design review B-M3). Gone with
 * the focus, a reader by now, it gives the focus away (left) before it
 * leaves the document.
 */
export function RestoreButton({
  label,
  text,
  restore,
  left,
}: {
  label: string;
  /** What it says; Restore without it. */
  text?: string;
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
      {text ?? t("trash.restore")}
    </Button>
  );
}

/**
 * TrashRow is an item of the trash's list (M8/P2 design 4.2): its kind's
 * icon, its name, which leads to its preview, and what is said of it
 * (TrashFacts); a writer's Restore. Its link and Restore name it by where
 * it was, a twin by when it was deleted too, and one of the same minute by
 * the end of its id, which its row shows. Gone with the focus,
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
  /** How it is told from the items listed of its name and place. */
  twin: Twin;
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
        <TrashFacts item={item} now={now} id={twin === "id" ? shortId(item.id) : undefined} />
      </div>
      {writesPages(notebook.role) && <RestoreButton label={names.restore} restore={restore} left={left} />}
    </li>
  );
});
