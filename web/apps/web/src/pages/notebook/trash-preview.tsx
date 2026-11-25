import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { Link, Navigate, useParams } from "react-router";
import useSWR from "swr";

import { arrived, useArrivalFocus, useRouteNotice } from "../../app/arrival";
import { ColumnTop } from "../../app/column-top";
import { useDocumentTitle } from "../../app/document-title";
import { writesNotebook } from "../../app/effective-role";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { docFactsClass, titleTextClass } from "../../app/page-classes";
import { Alert } from "../../components/ui/alert";
import { formatBytes } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { assetKind } from "../../lib/asset-kind";
import { cn } from "../../lib/cn";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { idPattern } from "../../lib/ids";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { TrashItem } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { NotFoundPage } from "../not-found";
import { openOf } from "../open-node";
import { BreadcrumbTrail } from "../page/breadcrumbs";
import {
  docColumnClass,
  docShellClass,
  docTitleClass,
  EmptyHead,
  headCrumbsClass,
  headRowClass,
  PageColumn,
} from "../page/page-layout";
import { useLeftBehind, VersionView } from "../page/version-view";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNotebook } from "./notebook-layout";
import { sourceHistoryText } from "./duplicate-text";
import { useNodeNavigate } from "../node-target";
import {
  backgroundClass,
  ItemIcon,
  namesOf,
  offeredOf,
  refusedText,
  RestoreButton,
  TrashFacts,
  trashRestoredOf,
  useBackgroundRestore,
  useTrashRestore,
  type RestoredState,
  type Started,
  type StartedState,
  type TrashRestored,
} from "./trash-row";

/**
 * TrashPreview is a trash item's preview (M8 design 4.5; M8/P2 design 4.3),
 * by its root's node in the address: what the trash says of it and its
 * root, a page's reading view at the revision it was deleted at, read only
 * (version-view.tsx), or an attachment's name, type and size, which is not
 * downloaded before it is restored; the rest of the item by its counts
 * alone. Its view leaves SWR's cache as the preview is left
 * (useLeftBehind): a few dozen previews keep none of their HTML. An
 * address of no node's id, or an item of another notebook, is no page of
 * this one's; a node deleted with a page above it goes to its item's
 * root's preview, which says why; one restored or past its time says so,
 * with where to go. A writer restores it here, as from the list, and goes
 * back to the trash, which says where it went; refused as too large, in
 * the background, and goes back to the trash, which says it started (M8/P4
 * design 4.2, review B-M5): the item's first batch would have it read as
 * no longer in the trash here.
 */
export const TrashPreview = observer(function TrashPreview() {
  const notebook = useNotebook();
  const { nodeId = "" } = useParams();
  // Each branch in the column, below an empty row of the trail alone, as the item read has it (decisions 915, 951).
  if (!idPattern.test(nodeId)) {
    return (
      <PageColumn>
        <NotFoundPage />
      </PageColumn>
    );
  }
  // As the server writes it, which the tree and the store key by: a node restored is found (M5-polish 23.19).
  const id = nodeId.toLowerCase();
  return <PreviewShell key={id} notebook={notebook} id={id} />;
});

/**
 * What came of a restore from the preview: out, the item restored, or
 * refused, or refused as deleted again with a page above it, whose item's
 * preview, root, says why (merged); of its restore in the background:
 * started, refused (start), or, its answer lost, not started or not known
 * to have (lost).
 */
type Restoring =
  | { out: true }
  | { restored: TrashRestored }
  | { refused: unknown; item: TrashItem; start?: boolean }
  | { merged: string }
  | { started: Started }
  | { lost: "none" | "unread"; item: TrashItem };

/**
 * refusedOf is what a restore of item refused came to: deleted again with a
 * page above it, its item's preview, which says so, and says it once
 * (M5-polish 22.19): the refusal's own alert, then the preview's, then the
 * title described by it, said it three times. That needs no item; another
 * refusal is said of the item, nothing without one (not read yet).
 */
function refusedOf(refusal: unknown, item: TrashItem | undefined): Restoring | undefined {
  const root =
    refusal instanceof ApiError && refusal.code === "page.not_trash_root" ? refusal.problem?.root_id : undefined;
  if (typeof root === "string") {
    return { merged: root };
  }
  return item === undefined ? undefined : { refused: refusal, item };
}

/**
 * MergedState is the route state of an item's preview gone to from a node
 * deleted with it since (not_trash_root): not arrived at, as the reader may
 * be elsewhere, others having deleted it (M8/P2B fix check 1, N1).
 */
type MergedState = { merged: true };

/** mergedOf is true when the route state says the preview was gone to from a node of its item. */
function mergedOf(state: unknown): true | undefined {
  return typeof state === "object" && state !== null && "merged" in state && state.merged === true ? true : undefined;
}

/**
 * PreviewShell is the preview of the node id, for as long as the address
 * has it: it holds what a restore from it came to, over what the item
 * reads meanwhile. While the restore is out, the item shown stays, though
 * a read says it is gone: the restore's own event reads it again, which
 * may answer first that it is no longer in the trash (M8/P2B review A-I1).
 * Restored, it goes back to the trash, arrived at, the item restored in
 * the route state, unless the user left meanwhile; refused, what the item
 * reads then shows, the refusal said above it (above what says it is gone,
 * when it is what said so), and what replaces the item shown takes the
 * focus it held. The refusal is one alert, kept as what shows below it
 * changes, which describes its title: announced once (M8/P2B fix check 1,
 * N2). Refused as too large, the item is offered a restore in the
 * background, as on the trash's list: out, the item shown stays as the
 * restore's own; started, it goes back to the trash, which says so; its
 * start refused, or its answer lost, the alert says what came of it, the
 * restore still offered where the trash's list would offer it. A restore
 * of the item out as it shows, sent from where the reader went back and
 * forward from, it follows as its own: shown as out, its Restore busy, then
 * gone to the trash or refused (M5-polish 22.14).
 */
const PreviewShell = observer(function PreviewShell({ notebook, id }: { notebook: Notebook; id: string }) {
  const pages = usePageTree(notebook);
  const navigateNode = useNodeNavigate(pages);
  const t = useT();
  const { slug } = useWorkspace();
  const mounted = useMounted();
  const restore = useTrashRestore(notebook);
  const start = useBackgroundRestore(notebook);
  useLeftBehind(notebook.id, id);
  // Said once, as the preview shows (M5-polish 22.2): kept here, the item's view remounting as it changes.
  const merged = useRouteNotice(mergedOf, ["merged"]) ?? false;
  const { data, error, mutate } = useSWR(["trash-item", notebook.id, id], () => pages.trashItem(id));
  // A restore out as the preview shows, sent before it did: out from its first render.
  const [restoring, setRestoring] = useState<Restoring | undefined>(() =>
    pages.restoreOut(id) === undefined ? undefined : { out: true }
  );
  // The item last read, which a restore followed is said of.
  const latest = useRef(data);
  useEffect(() => {
    if (data !== undefined) {
      latest.current = data;
    }
  }, [data]);
  // Followed as the preview's own, once: this mount's own restore is not out yet as it mounts.
  useEffect(() => {
    const out = pages.restoreOut(id);
    if (out === undefined) {
      return undefined;
    }
    let showing = true;
    void (async () => {
      try {
        const restored = await out;
        if (showing) {
          setRestoring({ restored: trashRestoredOf(restored, pages, latest.current) });
        }
      } catch (refusal) {
        if (showing) {
          setRestoring(refusedOf(refusal, latest.current));
        }
      }
    })();
    return () => {
      showing = false;
    };
  }, [pages, id]);
  // Whether a restore in the background is out: what came before it stays said meanwhile.
  const [starting, setStarting] = useState(false);
  // Whether the item shown left the document holding the focus, which what replaces it takes.
  const [handed, setHanded] = useState(false);
  const note = useId();
  const home = `/${slug}/notebooks/${notebook.id}`;
  // What shows, below what a refusal says, below the item's breadcrumb (M5-polish 22.19): each in a slot of its own,
  // so that a view replacing another keeps the same alert. The breadcrumb in the head's row and the rest in the
  // column, as a page's (M13/P5/S2, decision 929); without it, that row empty, as on the item's way (decision 946).
  const shell = (told: ReactNode, view: ReactNode, trail?: ReactNode) => (
    <div className={docShellClass}>
      {trail === undefined ? <EmptyHead /> : <Trail left={() => setHanded(true)}>{trail}</Trail>}
      <div className={docColumnClass}>
        <ColumnTop />
        {told !== undefined && (
          <Alert id={note} className="mb-4">
            {told}
          </Alert>
        )}
        {view}
      </div>
    </div>
  );
  const describing = (told: ReactNode) => (told === undefined ? undefined : note);

  async function restoreItem(item: TrashItem): Promise<void> {
    setRestoring({ out: true });
    try {
      const restored = await restore(item);
      if (mounted()) {
        setRestoring({ restored });
      }
    } catch (refusal) {
      if (mounted()) {
        setRestoring(refusedOf(refusal, item));
      }
    }
  }

  async function startItem(item: TrashItem): Promise<void> {
    setStarting(true);
    try {
      const started = await start(item);
      if (mounted()) {
        setRestoring(
          "job" in started ? { started: { name: item.name, job: started.job.id } } : { lost: started.lost, item }
        );
      }
    } catch (refusal) {
      if (mounted()) {
        setRestoring({ refused: refusal, item, start: true });
      }
    } finally {
      setStarting(false);
    }
  }

  if (restoring !== undefined && "restored" in restoring) {
    return (
      <Navigate to={`${home}/trash`} state={{ ...arrived, restored: restoring.restored } satisfies RestoredState} />
    );
  }
  if (restoring !== undefined && "merged" in restoring) {
    return <Navigate replace to={`${home}/trash/${restoring.merged}`} state={{ merged: true } satisfies MergedState} />;
  }
  if (restoring !== undefined && "started" in restoring) {
    return <Navigate to={`${home}/trash`} state={{ ...arrived, started: restoring.started } satisfies StartedState} />;
  }
  // An item read of another notebook is no page of this one's, whatever is read since.
  if (data !== undefined && data.notebook_id !== notebook.id) {
    return (
      <PageColumn>
        <NotFoundPage />
      </PageColumn>
    );
  }
  const refused = restoring !== undefined && "refused" in restoring ? restoring : undefined;
  const refusal = refused && { error: refused.refused, item: refused.item, start: refused.start };
  const lostStart = restoring !== undefined && "lost" in restoring ? restoring : undefined;
  const said =
    refusal !== undefined ? (
      (sourceHistoryText(refusal.error, pages, navigateNode) ?? refusedText(refusal, t))
    ) : lostStart === undefined ? undefined : (
      <LostStart {...lostStart} />
    );
  // Gone, the item says the refusal that said so: not one before it, of what the item read then.
  const saidGone = (code: string) =>
    refusal?.error instanceof ApiError && refusal.error.code === code ? said : undefined;
  // What the server answers now, read again, comes before what it answered before, unless a restore from here is
  // out: pressed on the item shown, which SWR keeps through a read that fails.
  const out = (restoring !== undefined && "out" in restoring) || starting;
  const offered = writesNotebook(notebook) ? (lostStart?.item ?? offeredOf(refusal)) : undefined;
  if (error instanceof ApiError && !out) {
    switch (error.code) {
      case "page.not_found":
        return (
          <PageColumn>
            <NotFoundPage />
          </PageColumn>
        );
      case "page.not_trash_root": {
        const root = error.problem?.root_id;
        if (root !== undefined) {
          return <Navigate replace to={`${home}/trash/${root}`} state={{ merged: true } satisfies MergedState} />;
        }
        break;
      }
      case "page.not_in_trash": {
        const told = saidGone(error.code);
        return shell(
          told,
          <NotInTrash
            notebook={notebook}
            id={id}
            describedBy={describing(told)}
            focus={handed}
            left={() => setHanded(true)}
          />
        );
      }
      case "page.restore_expired": {
        const told = saidGone(error.code);
        // Refused here as past its time, the alert above says it: the item's own sentence would say it again.
        return shell(
          told,
          <Gone
            text={told === undefined ? t("trash.expiredItem") : undefined}
            describedBy={describing(told)}
            focus={handed}
            left={() => setHanded(true)}
          />
        );
      }
    }
  }
  if (data === undefined) {
    return (
      <PageColumn>
        <NotLoaded error={error} retry={() => void mutate()} />
      </PageColumn>
    );
  }
  return shell(
    said,
    <Preview
      notebook={notebook}
      item={data}
      restore={() => restoreItem(data)}
      busy={restoring !== undefined && "out" in restoring}
      start={offered?.id === data.id ? () => startItem(data) : undefined}
      describedBy={describing(said)}
      merged={merged}
      focus={handed}
      left={() => setHanded(true)}
    />,
    <BreadcrumbTrail
      steps={[
        { key: "notebook", to: home, name: notebook.name },
        { key: "trash", to: `${home}/trash`, name: t("trash.title") },
      ]}
      current={data.name}
    />
  );
});

/**
 * Trail holds the item's breadcrumb, which leaves with the item: gone from
 * the document with the focus, it hands the focus on (left), as the item
 * does (M5-polish 22.19). It is the head's row, the trail's block alone
 * in it (M13/P5/S2, decision 951).
 */
function Trail({ left, children }: { left: () => void; children: ReactNode }) {
  const own = useRef<HTMLDivElement>(null);
  useFocusLeaving(own, left);
  return (
    <div ref={own} className={headRowClass}>
      <div className={headCrumbsClass}>{children}</div>
    </div>
  );
}

/**
 * LostStart says what came of the restore of item in the background whose
 * answer was lost: none started since it was pressed, or the jobs could not
 * be read, which lead there.
 */
function LostStart({ lost, item }: { lost: "none" | "unread"; item: TrashItem }) {
  const t = useT();
  const { slug } = useWorkspace();
  if (lost === "none") {
    return t("trash.backgroundNotStarted", { name: item.name });
  }
  return (
    <>
      {t("trash.backgroundUnknown", { name: item.name })}{" "}
      <Link
        to={`/${slug}/notebooks/${item.notebook_id}/settings/transfer`}
        state={arrived}
        className="underline underline-offset-4"
      >
        {t("transfer.seeJobs")}
      </Link>
    </>
  );
}

/**
 * Preview is the item found: its title, what the trash says of it, its
 * root's view or its attachment, and a writer's Restore (restore; what a
 * refusal says, above, describes its title: describedBy), and its Restore
 * in the background where it is offered (start); where it is, under the
 * trash, is its shell's (PreviewShell). Gone to from a node deleted with it
 * since, it says so, its title taking the focus if the reader has it
 * nowhere (v0.1 design 13.2, item 26): their own restore refused, what held
 * it gone, or the address opened; so does it shown in place of what held
 * the focus as it left (focus). Gone from the document with the focus,
 * read away, it hands the focus on (left).
 */
const Preview = observer(function Preview({
  notebook,
  item,
  restore,
  busy,
  start,
  describedBy,
  merged,
  focus,
  left,
}: {
  notebook: Notebook;
  item: TrashItem;
  restore: () => Promise<void>;
  /** Whether a restore of the item is out: one sent before the preview showed, its Restore busy. */
  busy: boolean;
  start: (() => Promise<void>) | undefined;
  describedBy: string | undefined;
  /** Whether it was gone to from a node deleted with it since. */
  merged: boolean;
  /** Whether what it replaces left with the focus. */
  focus: boolean;
  left: () => void;
}) {
  const t = useT();
  const { preferences } = useStore();
  // Arrived at, from the settings' Back say, its title takes the focus (M13/P3/S1, decision 835).
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const note = useId();
  const shown = useRef<HTMLDivElement>(null);
  useFocusLeaving(shown, left);
  useEffect(() => {
    const at = document.activeElement;
    if ((merged || focus) && (at === null || at === document.body)) {
      heading.current?.focus();
    }
  }, [merged, focus, heading]);
  useDocumentTitle(item.name, t("trash.title"), notebook.name);
  return (
    <div ref={shown} className="space-y-6">
      <div className="space-y-2">
        <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2">
          {/* The page's title, the width its buttons leave (P5B-M7); its name broken anywhere in it (decision 937). */}
          <h1
            ref={heading}
            tabIndex={-1}
            aria-describedby={describedBy ?? (merged ? note : undefined)}
            className={cn(docTitleClass, titleTextClass, "mb-0 flex items-start gap-2.5")}
          >
            <ItemIcon item={item} className="mt-1.5 size-6 shrink-0 text-faint" />
            <span className="min-w-0 wrap-anywhere">{item.name}</span>
          </h1>
          {writesNotebook(notebook) && (
            // On the right below the title too (P5B-Q2).
            <div className="ms-auto flex flex-wrap justify-end gap-2">
              <RestoreButton
                label={namesOf(item, "none", t, preferences.locale).restore}
                restore={restore}
                left={() => heading.current?.focus()}
                busy={busy}
              />
              {start !== undefined && (
                <RestoreButton
                  label={namesOf(item, "none", t, preferences.locale).background}
                  text={t("trash.background", { name: item.name })}
                  restore={start}
                  left={() => heading.current?.focus()}
                  className={backgroundClass}
                />
              )}
            </div>
          )}
        </div>
        {/* Spaced below the title and between their lines as a changeset's are (review B's P5S2B-N3). */}
        <TrashFacts item={item} now={Date.now()} className={cn(docFactsClass, "mt-3.5 space-y-1")} />
      </div>
      {merged && <Alert id={note}>{t("trash.mergedHere")}</Alert>}
      {item.kind === "page" ? <PagePreview notebook={notebook} item={item} /> : <AttachmentPreview item={item} />}
    </div>
  );
});

/** PagePreview is a page's view as it was deleted, read only, with what its links lead to; none without its content. */
function PagePreview({ notebook, item }: { notebook: Notebook; item: TrashItem }) {
  const t = useT();
  if (item.revision === null) {
    return <p className="text-muted-foreground">{t("trash.noPreview")}</p>;
  }
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">{t("trash.previewNote")}</p>
      <VersionView notebook={notebook} page={item.id} revision={item.revision} label={item.name} />
    </div>
  );
}

/** AttachmentPreview is an attachment's type and size, to download once restored; none without its file. */
function AttachmentPreview({ item }: { item: TrashItem }) {
  const t = useT();
  const { preferences } = useStore();
  if (item.asset === null) {
    return <p className="text-muted-foreground">{t("trash.noPreview")}</p>;
  }
  return (
    <div className="space-y-1">
      <p>
        {t(`asset.kind.${assetKind(item.asset.mime)}`)} · {formatBytes(item.asset.byte_size, preferences.locale)}
      </p>
      <p className="text-sm text-muted-foreground">{t("trash.downloadAfter")}</p>
    </div>
  );
}

/**
 * NotInTrash says the node is no longer in the trash, restored since, with
 * a way to it where the tree has it: a page to itself, an attachment to
 * where it is. A node the tree, read as this shows, does not have is
 * another notebook's: no page of this one's.
 */
const NotInTrash = observer(function NotInTrash({
  notebook,
  id,
  describedBy,
  focus,
  left,
}: {
  notebook: Notebook;
  id: string;
  describedBy: string | undefined;
  focus: boolean;
  left: () => void;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const [read, setRead] = useState(false);
  useEffect(() => {
    let showing = true;
    // A tree that could not be read says nothing of the node.
    void (async () => {
      try {
        await pages.load();
      } catch {
        return;
      }
      if (showing) {
        setRead(true);
      }
    })();
    return () => {
      showing = false;
    };
  }, [pages]);
  const node = pages.nodes?.find((each) => each.id === id);
  if (read && node === undefined) {
    return <NotFoundPage />;
  }
  const home = `/${slug}/notebooks/${notebook.id}`;
  const open = node === undefined ? undefined : openOf({ id, kind: node.kind, parent: node.parent_id }, home);
  // Before the tree is read, or where it could not be, the node may be this notebook's, restored, or another's: it
  // is not in the trash, without saying it no longer is (M12/P2/S2, 22.13).
  return (
    <Gone
      text={t(node === undefined ? "trash.notInTrashUnknown" : "trash.notInTrash")}
      describedBy={describedBy}
      focus={focus}
      left={left}
    >
      {open !== undefined && (
        <Link to={open.to} state={open.state} className="underline underline-offset-4">
          {t("trash.openIt")}
        </Link>
      )}
    </Gone>
  );
});

/**
 * Gone says what became of the item (text, unless the refusal above says
 * it), and leads back to the trash, or where children lead; what a restore
 * from the preview was refused for, above it (PreviewShell), describes its
 * title (describedBy). Shown in place of the item, which left with the
 * focus (focus), its title takes it, the focus on nothing. Gone from the
 * document with the focus, the item back in the trash, it hands the focus
 * on (left), which the item's title takes (M5-polish 22.19).
 */
function Gone({
  text,
  describedBy,
  focus,
  left,
  children,
}: {
  text: string | undefined;
  describedBy: string | undefined;
  focus: boolean;
  left: () => void;
  children?: ReactNode;
}) {
  const t = useT();
  const notebook = useNotebook();
  const { slug } = useWorkspace();
  // Arrived at, from the settings' Back say, its title takes the focus (M13/P3/S1, decision 835).
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const section = useRef<HTMLElement>(null);
  useFocusLeaving(section, left);
  useDocumentTitle(t("trash.title"), notebook.name);
  useEffect(() => {
    const at = document.activeElement;
    if (focus && (at === null || at === document.body)) {
      heading.current?.focus();
    }
  }, [focus, heading]);
  return (
    <section ref={section} className="space-y-2">
      <h1 ref={heading} tabIndex={-1} aria-describedby={describedBy} className={docTitleClass}>
        {t("trash.title")}
      </h1>
      {text !== undefined && <p>{text}</p>}
      <p className="flex flex-wrap gap-4">
        {children}
        <Link to={`/${slug}/notebooks/${notebook.id}/trash`} className="underline underline-offset-4">
          {t("trash.back")}
        </Link>
      </p>
    </section>
  );
}
