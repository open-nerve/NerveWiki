import { observer } from "mobx-react-lite";
import { useState, type ReactNode } from "react";
import { Link, Navigate, useNavigate, useParams } from "react-router";
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
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { TrashItem } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { NotFoundPage } from "../not-found";
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
} from "./trash-row";

/**
 * TrashPreview is a trash item's preview (M8 design 4.5; M8/P2 design 4.3),
 * by its root's node in the address: what the trash says of it and its
 * root, a page's reading view at the revision it was deleted at, read only
 * (version-view.tsx), or an attachment's name, type and size, which is not
 * downloaded before it is restored; the rest of the item by its counts
 * alone. Its view leaves SWR's cache as the preview is left
 * (useLeftBehind): a few dozen previews keep none of their HTML. An item
 * of another notebook is no page of this one's; a node deleted with a
 * page above it goes to its item's root's preview; one restored or past
 * its time says so, with where to go. A writer restores it here, as from
 * the list, and goes back to the trash, which says where it went.
 */
export const TrashPreview = observer(function TrashPreview() {
  const notebook = useNotebook();
  const { nodeId = "" } = useParams();
  return <PreviewShell key={nodeId} notebook={notebook} id={nodeId} />;
});

const PreviewShell = observer(function PreviewShell({ notebook, id }: { notebook: Notebook; id: string }) {
  const pages = usePageTree(notebook);
  const t = useT();
  const { slug } = useWorkspace();
  useLeftBehind(notebook.id, id);
  const { data, error, mutate } = useSWR(["trash-item", notebook.id, id], () => pages.trashItem(id));
  const home = `/${slug}/notebooks/${notebook.id}`;
  // What the server answers now, read again, comes before what it answered before.
  if (error instanceof ApiError) {
    switch (error.code) {
      case "page.not_found":
        return <NotFoundPage />;
      case "page.not_trash_root": {
        const root = error.problem?.root_id;
        if (root !== undefined) {
          return <Navigate replace to={`${home}/trash/${root}`} />;
        }
        break;
      }
      case "page.not_in_trash":
        return <NotInTrash notebook={notebook} id={id} />;
      case "page.restore_expired":
        return <Gone text={t("trash.expiredItem")} />;
    }
  }
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  if (data.notebook_id !== notebook.id) {
    return <NotFoundPage />;
  }
  return <Preview notebook={notebook} item={data} />;
});

/**
 * Preview is the item found: where it is, under the trash, its title, what
 * the trash says of it, its root's view or its attachment, and a writer's
 * Restore, which goes back to the trash, arrived at, once restored; a
 * refusal says why above, the item read again.
 */
const Preview = observer(function Preview({ notebook, item }: { notebook: Notebook; item: TrashItem }) {
  const t = useT();
  const { slug } = useWorkspace();
  const { preferences } = useStore();
  const navigate = useNavigate();
  const mounted = useMounted();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const trashTitle = t("trash.title");
  useDocumentTitle(item.name, trashTitle, notebook.name);
  const restore = useTrashRestore(notebook);
  const [refused, setRefused] = useState<unknown>(undefined);
  const home = `/${slug}/notebooks/${notebook.id}`;

  async function restoreItem(): Promise<void> {
    setRefused(undefined);
    try {
      const restored = await restore(item);
      // Back to the trash, arrived at, unless the user left meanwhile.
      if (mounted()) {
        void navigate(`${home}/trash`, { state: { ...arrived, restored } satisfies RestoredState });
      }
    } catch (error) {
      if (mounted()) {
        setRefused(error);
      }
    }
  }

  const failed = refused === undefined ? undefined : refusalText(refused, item, t);
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <nav aria-label={t("page.breadcrumb")}>
          <ol className="flex flex-wrap items-center gap-x-1 text-sm text-muted-foreground">
            <li>
              <Link to={home} className="hover:underline">
                {notebook.name}
              </Link>
            </li>
            <li>
              <span aria-hidden className="pr-1">
                /
              </span>
              <Link to={`${home}/trash`} className="hover:underline">
                {trashTitle}
              </Link>
            </li>
            <li className="text-foreground">
              <span aria-hidden className="pr-1 text-muted-foreground">
                /
              </span>
              <span aria-current="page">{item.name}</span>
            </li>
          </ol>
        </nav>
        <div className="flex items-start justify-between gap-3">
          <h1
            ref={heading}
            tabIndex={-1}
            className="flex min-w-0 items-center gap-2 text-3xl font-semibold break-words outline-none"
          >
            <ItemIcon item={item} />
            {item.name}
          </h1>
          {writesPages(notebook.role) && (
            <RestoreButton
              label={namesOf(item, false, t, preferences.locale).restore}
              restore={restoreItem}
              left={() => heading.current?.focus()}
            />
          )}
        </div>
        <div className="space-y-1">
          <TrashFacts item={item} now={Date.now()} />
        </div>
      </div>
      {failed !== undefined && <Alert>{failed}</Alert>}
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
 * where it is.
 */
const NotInTrash = observer(function NotInTrash({ notebook, id }: { notebook: Notebook; id: string }) {
  const t = useT();
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  useSWR(["pages", notebook.id], () => pages.load());
  const node = pages.nodes?.find((each) => each.id === id);
  const home = `/${slug}/notebooks/${notebook.id}`;
  const open = node === undefined ? undefined : openOf({ id, kind: node.kind, parent: node.parent_id }, home);
  return (
    <Gone text={t("trash.notInTrash")}>
      {open !== undefined && (
        <Link to={open.to} state={open.state} className="underline underline-offset-4">
          {t("trash.openIt")}
        </Link>
      )}
    </Gone>
  );
});

/** Gone says what became of the item, and leads back to the trash, or where children lead. */
function Gone({ text, children }: { text: string; children?: ReactNode }) {
  const t = useT();
  const notebook = useNotebook();
  const { slug } = useWorkspace();
  useDocumentTitle(t("trash.title"), notebook.name);
  return (
    <section className="space-y-2">
      <h1 className="text-2xl font-semibold">{t("trash.title")}</h1>
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
