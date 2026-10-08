import { observer } from "mobx-react-lite";
import { useCallback, useEffect, useId, useRef, useState } from "react";
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
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { EditLock, TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { EditEnded } from "../../stores/edit-session";
import type { PageEditing } from "../../stores/page-editing";
import { useNotebook } from "../notebook/notebook-layout";
import { NotFoundPage } from "../not-found";
import { useWorkspace } from "../workspace/workspace-layout";
import { AttachmentDrop, AttachmentsSection } from "./attachments-section";
import { Breadcrumbs } from "./breadcrumbs";
import { EditLockNote } from "./edit-lock-note";
import { PageEdit } from "./page-edit";
import { PagePanel } from "./page-panel";
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
  const found = pages.tree === undefined ? undefined : pages.byId(pageId);
  if (found !== undefined && found !== last) {
    setLast(found);
  }
  if (pages.tree === undefined) {
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
 * Edit; an edit left for a long time without input says so above the
 * reading view until the next (M5/P5 design 3.7). The edit is not in the
 * address: a reload shows the reading view. Below its children are its
 * attachments; files dropped on the reading view upload there (M7/P4
 * design 3.5, 3.6). Beside its content, or after
 * it on a narrow window, is its right column (M6/P7 design 7).
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
  const [idleLeft, setIdleLeft] = useState(false);
  // The note of an idle exit describes Edit, where the focus lands: it is heard as it comes.
  const idleNote = useId();
  const edit = useRef<HTMLButtonElement>(null);
  // The last navigation the reading view took in, which outlives it while the page is edited.
  const anchored = useRef<string | undefined>(undefined);
  const lockNote = useRef<HTMLDivElement>(null);
  const back = useRef(false);
  // The edit whose session is opening: one that opens once the shell is gone ends.
  const opening = useRef<PageEditing | undefined>(undefined);
  const mounted = useMounted();
  const writer = writesPages(notebook.role);
  const home = `/${slug}/notebooks/${notebook.id}`;
  // One for the notebook: the backlinks (an observer, memoized) render again not as the shell's own state changes (the
  // edit entered or left); the notebook and its tree read again still render them.
  const href = useCallback((id?: string) => (id === undefined ? home : `${home}/pages/${id}`), [home]);
  const children = pages.childrenOf(page.id);

  async function enter(takeOver: boolean): Promise<void> {
    if (opening.current !== undefined) {
      return;
    }
    const next = store.editPage(notebook, page.id);
    if (next === undefined) {
      return;
    }
    opening.current = next;
    setEntering(true);
    setRefusal(undefined);
    setIdleLeft(false);
    try {
      const opened = await next.begin(takeOver);
      if (!mounted()) {
        void next.end();
      } else if (opened.opened) {
        heading.current?.focus();
        setEditing(next);
      } else {
        // The note reads who holds the lock now, and takes the focus; a lock let go meanwhile leaves it on Edit.
        const lock = await mutate<EditLock>(["edit-lock", page.id]);
        (lock?.holder ? lockNote : edit).current?.focus();
      }
    } catch (error) {
      // An edit ended as it opens, the tab signing out, is no failure.
      if (mounted() && !(error instanceof EditEnded)) {
        setRefusal(error);
      }
    } finally {
      opening.current = undefined;
      if (mounted()) {
        setEntering(false);
      }
    }
  }

  // A task item's toggle refused (M5/P6 design 3.5): page.locked as Edit's refusal is, the note reading who holds
  // the lock and taking the focus from the checkbox, unless the focus has gone elsewhere meanwhile; another refusal
  // above the view. undefined, as a toggle starts, clears it.
  async function toggleRefused(error: unknown): Promise<void> {
    if (!(error instanceof ApiError && error.code === "page.locked")) {
      setRefusal(error);
      return;
    }
    const lock = await mutate<EditLock>(["edit-lock", page.id]);
    const focused = document.activeElement;
    if (
      mounted() &&
      lock?.holder &&
      (focused === null || focused === document.body || focused.matches("input[data-task]"))
    ) {
      lockNote.current?.focus();
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
              aria-describedby={idleLeft ? idleNote : undefined}
              aria-disabled={entering || undefined}
              onClick={() => void enter(false)}
            >
              {t("page.edit")}
            </Button>
          )}
        </div>
      </div>
      <div className="space-y-6 xl:grid xl:grid-cols-[minmax(0,1fr)_15rem] xl:items-start xl:gap-8 xl:space-y-0">
        <div className="min-w-0 space-y-6">
          {editing === undefined ? (
            <>
              {refusal !== undefined && <Alert>{errorText(refusal, t)}</Alert>}
              {idleLeft && (
                <output id={idleNote} className="block text-sm text-muted-foreground">
                  {t("page.idleLeft")}
                </output>
              )}
              <div ref={lockNote} tabIndex={-1} className="outline-none">
                <EditLockNote
                  notebook={notebook}
                  page={page}
                  editHere={writer ? () => void enter(true) : undefined}
                  released={() => (edit.current ?? heading.current)?.focus()}
                />
              </div>
              <AttachmentDrop notebook={notebook} parent={page.id} enabled={writer}>
                <ReadingView
                  notebook={notebook}
                  page={page}
                  refused={(error) => (error === undefined ? setRefusal(undefined) : void toggleRefused(error))}
                  unanchored={() => heading.current?.focus()}
                  anchored={anchored}
                />
              </AttachmentDrop>
            </>
          ) : (
            <PageEdit
              notebook={notebook}
              page={page}
              editing={editing}
              done={(left) => {
                back.current = true;
                setIdleLeft(left.idle);
                // A toggle's refusal that came while it edited is no longer news.
                setRefusal(undefined);
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
          <AttachmentsSection notebook={notebook} parent={page.id} />
        </div>
        <PagePanel notebook={notebook} page={page} editing={!reading} href={href} />
      </div>
    </div>
  );
});
