import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { Link, Navigate, useLocation, useParams } from "react-router";
import useSWR from "swr";

import { arrived, useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { writesPages } from "../../app/effective-role";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { Alert } from "../../components/ui/alert";
import { formatBytes } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { assetKind } from "../../lib/asset-kind";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { TrashItem } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { NotFoundPage } from "../not-found";
import { BreadcrumbTrail } from "../page/breadcrumbs";
import { useLeftBehind, VersionView } from "../page/version-view";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNotebook } from "./notebook-layout";
import {
  ItemIcon,
  namesOf,
  openOf,
  refusalText,
  RestoreButton,
  TrashFacts,
  useTrashRestore,
  type RestoredState,
  type TrashRestored,
} from "./trash-row";

/** nodeIdPattern is a node's id, a UUID: anything else in the address is no item, not a read refused as malformed. */
const nodeIdPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/iu;

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
 * back to the trash, which says where it went.
 */
export const TrashPreview = observer(function TrashPreview() {
  const notebook = useNotebook();
  const { nodeId = "" } = useParams();
  if (!nodeIdPattern.test(nodeId)) {
    return <NotFoundPage />;
  }
  return <PreviewShell key={nodeId} notebook={notebook} id={nodeId} />;
});

/** What came of a restore from the preview: out, the item restored, or refused. */
type Restoring = { out: true } | { restored: TrashRestored } | { refused: unknown; item: TrashItem };

/** MergedState is the route state of an item's preview gone to from a node deleted with it since (not_trash_root). */
type MergedState = { arrived: true; merged: true };

/** mergedOf tells whether the route state says the preview was gone to from a node of its item. */
function mergedOf(state: unknown): boolean {
  return typeof state === "object" && state !== null && "merged" in state && state.merged === true;
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
 * focus it held.
 */
const PreviewShell = observer(function PreviewShell({ notebook, id }: { notebook: Notebook; id: string }) {
  const pages = usePageTree(notebook);
  const t = useT();
  const { slug } = useWorkspace();
  const mounted = useMounted();
  const restore = useTrashRestore(notebook);
  useLeftBehind(notebook.id, id);
  const { data, error, mutate } = useSWR(["trash-item", notebook.id, id], () => pages.trashItem(id));
  const [restoring, setRestoring] = useState<Restoring>();
  // Whether the item shown left the document holding the focus, which what replaces it takes.
  const [handed, setHanded] = useState(false);
  const home = `/${slug}/notebooks/${notebook.id}`;

  async function restoreItem(item: TrashItem): Promise<void> {
    setRestoring({ out: true });
    try {
      const restored = await restore(item);
      if (mounted()) {
        setRestoring({ restored });
      }
    } catch (refusal) {
      if (mounted()) {
        setRestoring({ refused: refusal, item });
      }
    }
  }

  if (restoring !== undefined && "restored" in restoring) {
    return (
      <Navigate to={`${home}/trash`} state={{ ...arrived, restored: restoring.restored } satisfies RestoredState} />
    );
  }
  // An item read of another notebook is no page of this one's, whatever is read since.
  if (data !== undefined && data.notebook_id !== notebook.id) {
    return <NotFoundPage />;
  }
  const refusal = restoring !== undefined && "refused" in restoring ? restoring : undefined;
  const said = refusal === undefined ? undefined : refusalText(refusal.refused, refusal.item, t);
  // Gone, the item says the refusal that said so: not one before it, of what the item read then.
  const saidGone = (code: string) =>
    refusal?.refused instanceof ApiError && refusal.refused.code === code ? said : undefined;
  // What the server answers now, read again, comes before what it answered before, unless a restore from here is
  // out: pressed on the item shown, which SWR keeps through a read that fails.
  const out = restoring !== undefined && "out" in restoring;
  if (error instanceof ApiError && !out) {
    switch (error.code) {
      case "page.not_found":
        return <NotFoundPage />;
      case "page.not_trash_root": {
        const root = error.problem?.root_id;
        if (root !== undefined) {
          return (
            <Navigate replace to={`${home}/trash/${root}`} state={{ ...arrived, merged: true } satisfies MergedState} />
          );
        }
        break;
      }
      case "page.not_in_trash":
        return <NotInTrash notebook={notebook} id={id} said={saidGone(error.code)} focus={handed} />;
      case "page.restore_expired":
        return <Gone text={t("trash.expiredItem")} said={saidGone(error.code)} focus={handed} />;
    }
  }
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  return (
    <Preview
      notebook={notebook}
      item={data}
      restore={() => restoreItem(data)}
      said={said}
      left={() => setHanded(true)}
    />
  );
});

/**
 * Preview is the item found: where it is, under the trash, its title, what
 * the trash says of it, its root's view or its attachment, and a writer's
 * Restore (restore, which says what came of it: said, a refusal, above).
 * Gone to from a node deleted with it since, it says so, its title taking
 * the focus. Gone from the document with the focus, read away, it hands
 * the focus on (left).
 */
const Preview = observer(function Preview({
  notebook,
  item,
  restore,
  said,
  left,
}: {
  notebook: Notebook;
  item: TrashItem;
  restore: () => Promise<void>;
  said: string | undefined;
  left: () => void;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const { preferences } = useStore();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const merged = mergedOf(useLocation().state);
  const note = useId();
  const shown = useRef<HTMLDivElement>(null);
  useFocusLeaving(shown, left);
  const trashTitle = t("trash.title");
  useDocumentTitle(item.name, trashTitle, notebook.name);
  const home = `/${slug}/notebooks/${notebook.id}`;
  const alert = said ?? (merged ? t("trash.mergedHere") : undefined);
  return (
    <div ref={shown} className="space-y-6">
      <div className="space-y-2">
        <BreadcrumbTrail
          steps={[
            { key: "notebook", to: home, name: notebook.name },
            { key: "trash", to: `${home}/trash`, name: trashTitle },
          ]}
          current={item.name}
        />
        <div className="flex items-start justify-between gap-3">
          <h1
            ref={heading}
            tabIndex={-1}
            aria-describedby={alert === undefined ? undefined : note}
            className="flex min-w-0 items-center gap-2 text-3xl font-semibold break-words outline-none"
          >
            <ItemIcon item={item} />
            {item.name}
          </h1>
          {writesPages(notebook.role) && (
            <RestoreButton
              label={namesOf(item, "none", t, preferences.locale).restore}
              restore={restore}
              left={() => heading.current?.focus()}
            />
          )}
        </div>
        <div className="space-y-1">
          <TrashFacts item={item} now={Date.now()} />
        </div>
      </div>
      {alert !== undefined && <Alert id={note}>{alert}</Alert>}
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
  said,
  focus,
}: {
  notebook: Notebook;
  id: string;
  said: string | undefined;
  focus: boolean;
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
  return (
    <Gone text={t("trash.notInTrash")} said={said} focus={focus}>
      {open !== undefined && (
        <Link to={open.to} state={open.state} className="underline underline-offset-4">
          {t("trash.openIt")}
        </Link>
      )}
    </Gone>
  );
});

/**
 * Gone says what became of the item, and leads back to the trash, or where
 * children lead; what a restore from the preview was refused for (said)
 * above it, announced and describing its title. Shown in place of the
 * item, which left with the focus (focus), its title takes it.
 */
function Gone({
  text,
  said,
  focus,
  children,
}: {
  text: string;
  said: string | undefined;
  focus: boolean;
  children?: ReactNode;
}) {
  const t = useT();
  const notebook = useNotebook();
  const { slug } = useWorkspace();
  const heading = useRef<HTMLHeadingElement>(null);
  const note = useId();
  useDocumentTitle(t("trash.title"), notebook.name);
  useEffect(() => {
    const at = document.activeElement;
    if (focus && (at === null || at === document.body)) {
      heading.current?.focus();
    }
  }, [focus]);
  return (
    <section className="space-y-2">
      <h1
        ref={heading}
        tabIndex={-1}
        aria-describedby={said === undefined ? undefined : note}
        className="text-2xl font-semibold outline-none"
      >
        {t("trash.title")}
      </h1>
      {said !== undefined && <Alert id={note}>{said}</Alert>}
      <p>{text}</p>
      <p className="flex flex-wrap gap-4">
        {children}
        <Link to={`/${slug}/notebooks/${notebook.id}/trash`} className="underline underline-offset-4">
          {t("trash.back")}
        </Link>
      </p>
    </section>
  );
}
