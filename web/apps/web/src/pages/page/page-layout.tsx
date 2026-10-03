import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { Navigate, useParams } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { arrived, useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { writesPages } from "../../app/effective-role";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { dialogOpen, isMod, onMac } from "../../app/shortcuts";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import type { PageEditing } from "../../stores/page-editing";
import { useNotebook } from "../notebook/notebook-layout";
import { NotFoundPage } from "../not-found";
import { useWorkspace } from "../workspace/workspace-layout";
import { Breadcrumbs } from "./breadcrumbs";
import { EditLockNote } from "./edit-lock-note";
import { PageEdit } from "./page-edit";
import { ReadingView } from "./reading-view";
import { SubpageList } from "./subpage-list";

/**
 * PageLayout is a page's shell (M4/P5 design 3.5). It finds the page of the
 * address in the notebook's tree, as the left column shows it: until the
 * tree is read it shows nothing of the page; a page the tree does not
 * have is no page of the app's, unless this tab deleted it: its shell then
 * goes to the deleted subtree's parent, or the notebook's home when that
 * is gone too, arrived at. The page's title, ancestors and children come
 * from the tree too, so that the tree read again refreshes them. A page
 * gone while this tab edits it with changes not saved stays, as it was
 * last found, until the edit ends: what was typed can be copied (M5/P4
 * design 3.9).
 */
export const PageLayout = observer(function PageLayout() {
  const notebook = useNotebook();
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const store = useStore();
  const { pageId = "" } = useParams();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const [last, setLast] = useState<TreeNode | undefined>(undefined);
  const found = pages.nodes === undefined ? undefined : pages.byId(pageId);
  if (found !== undefined && found !== last) {
    setLast(found);
  }
  if (pages.nodes === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const page = found ?? (last?.id === pageId && store.unsavedEdit({ pageId }) ? last : undefined);
  if (page === undefined) {
    const to = pages.removedTo(pageId);
    if (to === undefined) {
      return <NotFoundPage />;
    }
    const home = `/${slug}/notebooks/${notebook.id}`;
    return (
      <Navigate
        replace
        to={to !== null && pages.byId(to) !== undefined ? `${home}/pages/${to}` : home}
        state={arrived}
      />
    );
  }
  return <PageShell key={page.id} notebook={notebook} page={page} />;
});

/**
 * PageShell is the page found: where it is, its title, who is editing it
 * (M5/P3 design 3.10), its reading view and its children. A writer edits
 * it in its place (M4/P6 design 3.7; M5/P4 design 3.6): Edit or Mod+E
 * opens the edit's session first, Edit busy meanwhile; once the lock is
 * the edit's, the reading view leaves for the editor, the focus on the
 * page's title until the editor takes it. A lock someone holds keeps the
 * reading view, the focus on who holds it; the account itself elsewhere
 * may edit here, taking it over. Back from the edit, the focus is on
 * Edit. The edit is not in the address: a reload shows the reading view.
 */
const PageShell = observer(function PageShell({ notebook, page }: { notebook: Notebook; page: TreeNode }) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const store = useStore();
  const t = useT();
  const { mutate } = useSWRConfig();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  useDocumentTitle(page.name, notebook.name);
  const [editing, setEditing] = useState<PageEditing | undefined>(undefined);
  const [entering, setEntering] = useState(false);
  const [refusal, setRefusal] = useState<unknown>(undefined);
  const edit = useRef<HTMLButtonElement>(null);
  const lockNote = useRef<HTMLDivElement>(null);
  const back = useRef(false);
  // The edit whose session is opening: one that opens once the shell is gone ends.
  const opening = useRef<PageEditing | undefined>(undefined);
  const mounted = useMounted();
  const writer = writesPages(notebook.role);
  const home = `/${slug}/notebooks/${notebook.id}`;
  const href = (id?: string) => (id === undefined ? home : `${home}/pages/${id}`);
  const children = pages.childrenOf(page.id);

  async function enter(takeOver: boolean): Promise<void> {
    if (opening.current !== undefined) {
      return;
    }
    const next = store.editPage(notebook.id, page.id);
    if (next === undefined) {
      return;
    }
    opening.current = next;
    setEntering(true);
    setRefusal(undefined);
    try {
      const opened = await next.begin(takeOver);
      if (!mounted()) {
        void next.end();
      } else if (opened.opened) {
        heading.current?.focus();
        setEditing(next);
      } else {
        // The note reads who holds the lock now, and takes the focus.
        await mutate(["edit-lock", page.id]);
        lockNote.current?.focus();
      }
    } catch (error) {
      if (mounted()) {
        setRefusal(error);
      }
    } finally {
      opening.current = undefined;
      if (mounted()) {
        setEntering(false);
      }
    }
  }

  // The keys of the latest render: enter reads its page.
  const keys = useRef({ enter });
  useEffect(() => {
    keys.current = { enter };
  });
  const reading = editing === undefined;
  useEffect(() => {
    if (!writer || !reading) {
      return undefined;
    }
    if (back.current) {
      back.current = false;
      edit.current?.focus();
    }
    const mac = onMac();
    const onKeyDown = (event: KeyboardEvent) => {
      if (isMod(event, "e", mac) && !dialogOpen()) {
        event.preventDefault();
        // A key held down acts once: the edit just left is not entered again.
        if (!event.repeat) {
          void keys.current.enter(false);
        }
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [writer, reading]);
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Breadcrumbs notebook={notebook} ancestors={pages.ancestorsOf(page.id)} page={page} href={href} />
        <div className="flex items-start justify-between gap-3">
          <h1 ref={heading} tabIndex={-1} className="text-3xl font-semibold break-words outline-none">
            {page.name}
          </h1>
          {writer && reading && (
            <Button
              ref={edit}
              variant="outline"
              aria-busy={entering || undefined}
              aria-disabled={entering || undefined}
              onClick={() => void enter(false)}
            >
              {t("page.edit")}
            </Button>
          )}
        </div>
      </div>
      {editing === undefined ? (
        <>
          {refusal !== undefined && <Alert>{errorText(refusal, t)}</Alert>}
          <div ref={lockNote} tabIndex={-1} className="outline-none">
            <EditLockNote notebook={notebook} page={page} editHere={writer ? () => void enter(true) : undefined} />
          </div>
          <ReadingView notebook={notebook} page={page} />
        </>
      ) : (
        <PageEdit
          notebook={notebook}
          page={page}
          editing={editing}
          done={() => {
            back.current = true;
            setEditing(undefined);
          }}
        />
      )}
      {children.length > 0 && (
        <section className="space-y-2">
          <h2 className="text-sm font-medium text-muted-foreground">{t("page.subpages")}</h2>
          <SubpageList label={t("page.subpages")} pages={children} href={href} />
        </section>
      )}
    </div>
  );
});
