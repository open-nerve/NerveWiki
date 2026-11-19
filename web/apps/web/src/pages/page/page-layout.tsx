import { observer } from "mobx-react-lite";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { Navigate, useNavigation, useParams } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { arrived, useArrivalFocus, useRouteNotice } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { writesNotebook } from "../../app/effective-role";
import { canEditPageBody } from "../../app/source-permissions";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { dialogOpen, isMod, onMac } from "../../app/shortcuts";
import { Loading } from "../../components/loading";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT, type Translate } from "../../i18n/i18n";
import { focusLost } from "../../lib/focus-leaving";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { EditLock, TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { EditEnded } from "../../stores/edit-session";
import type { PageEditing } from "../../stores/page-editing";
import { useNotebook } from "../notebook/notebook-layout";
import { NotFoundPage } from "../not-found";
import { useWorkspace } from "../workspace/workspace-layout";
import { AttachmentDrop } from "./attachment-drop";
import { AttachmentsSection } from "./attachments-section";
import { Breadcrumbs } from "./breadcrumbs";
import { EditLockNote } from "./edit-lock-note";
import { PageEdit } from "./page-edit";
import { PageMenu } from "./page-menu";
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
  const pages = usePageTree(notebook);
  const store = useStore();
  const { pageId = "" } = useParams();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const [last, setLast] = useState<TreeNode | undefined>(undefined);
  // A tree read before the notebook was left is read again before the page shows (M5-polish 27.2), saying
  // nothing of a read that failed then: SWR keeps its error, no longer trying again once nothing showed the tree.
  const [stale] = useState(() => (pages.confirmed ? undefined : error));
  useEffect(() => {
    if (!pages.confirmed && !pages.confirming()) {
      void mutate();
    }
  }, [pages, mutate]);
  const found = pages.tree === undefined || !pages.confirmed ? undefined : pages.byId(pageId);
  if (found !== undefined && found !== last) {
    setLast(found);
  }
  if (pages.tree === undefined || !pages.confirmed) {
    return <NotLoaded error={error === stale ? undefined : error} retry={() => void mutate()} />;
  }
  const page = found ?? (last?.id === pageId && store.unsavedEdit({ pageId }) ? last : undefined);
  if (page === undefined) {
    // In place of the page shown, deleted under its edit: "Page not found" takes the focus it took with it.
    return <PageGone notebook={notebook} page={pageId} replacing={last?.id === pageId} />;
  }
  return <PageShell key={page.id} notebook={notebook} page={page} gone={found === undefined} />;
});

/**
 * PageGone answers the page id the notebook's tree does not have, a page's
 * shell or a part of it: no page of the app's, unless this tab deleted it;
 * it then goes to the deleted subtree's parent, or the notebook's home
 * when that is gone too, arrived at; unless the reader is going elsewhere
 * meanwhile, which a replace would cancel (M5-polish 2): it then waits for
 * that navigation, which takes the page away whether it ends or not. In
 * place of the page this navigation showed (replacing), "Page not found"
 * takes the focus from no one (M12/P2/S1, 18.6).
 */
export const PageGone = observer(function PageGone({
  notebook,
  page,
  replacing = false,
}: {
  notebook: Notebook;
  page: string;
  replacing?: boolean;
}) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const navigation = useNavigation();
  const to = pages.removedTo(page);
  if (to === undefined) {
    return <NotFoundPage replacing={replacing} />;
  }
  if (navigation.state !== "idle") {
    return <Loading />;
  }
  const home = `/${slug}/notebooks/${notebook.id}`;
  return (
    <Navigate replace to={to !== null && pages.byId(to) !== undefined ? `${home}/pages/${to}` : home} state={arrived} />
  );
});

/** Restored is the route state of a page arrived at from its version restored (M8 design 4.4): the version's revision. */
export type Restored = { arrived: true; restored: number };

function restoredOf(state: unknown): number | undefined {
  return typeof state === "object" && state !== null && "restored" in state && typeof state.restored === "number"
    ? state.restored
    : undefined;
}

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
 * design 3.5, 3.6). A page gone, shown while its edit is unsaved, has none:
 * its list is not read. Beside its content, or after
 * it on a narrow window, is its right column (M6/P7 design 7). Arrived at
 * from one of its versions restored, it says so, with the title that has
 * the focus.
 */
const PageShell = observer(function PageShell({
  notebook,
  page,
  gone,
}: {
  notebook: Notebook;
  page: TreeNode;
  /** Whether the page has gone from the tree, shown still while its edit is unsaved: it has no attachments then. */
  gone: boolean;
}) {
  const workspace = useWorkspace();
  const { slug } = workspace;
  const pages = usePageTree(notebook);
  const store = useStore();
  const t = useT();
  const { mutate } = useSWRConfig();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  useDocumentTitle(page.name, notebook.name);
  // Said as the page shows, once: an edit entered takes it away (M5-polish 21.3).
  const notice = useRouteNotice(restoredOf, ["restored"]);
  const [restored, setRestored] = useState(notice);
  // The status of a version restored describes the title, where the focus lands: it is heard as it comes.
  const restoredNote = useId();
  const [editing, setEditing] = useState<PageEditing | undefined>(undefined);
  const [entering, setEntering] = useState(false);
  const [refusal, setRefusal] = useState<unknown>(undefined);
  const [idleLeft, setIdleLeft] = useState(false);
  // What the editor told as the edit waited to be left for its uploads: files not inserted.
  const [toldLeft, setToldLeft] = useState("");
  // The note of an edit's end, idle or told, describes Edit, where the focus lands: it is heard as it comes.
  const idleNote = useId();
  const edit = useRef<HTMLButtonElement>(null);
  // The last navigation the reading view took in, which outlives it while the page is edited.
  const anchored = useRef<string | undefined>(undefined);
  const lockNote = useRef<HTMLDivElement>(null);
  // Back from the edit, the focus goes to Edit; from one that waited for its uploads ("fallen"), only if it fell with the
  // edit gone, the user free to go elsewhere meanwhile.
  const back = useRef<boolean | "fallen">(false);
  // The edit whose session is opening: one that opens once the shell is gone ends.
  const opening = useRef<PageEditing | undefined>(undefined);
  // Whether the notebooks are read again after a refusal as forbidden (roleTaken).
  const roleRead = useRef(false);
  const mounted = useMounted();
  const writer = writesNotebook(notebook);
  const editable = canEditPageBody(pages, page.id);
  const source = pages.isSource(page.id);
  const home = `/${slug}/notebooks/${notebook.id}`;
  // One for the notebook: the backlinks (an observer, memoized) render again not as the shell's own state changes (the
  // edit entered or left); the notebook and its tree read again still render them.
  const href = useCallback((id?: string) => (id === undefined ? home : `${home}/pages/${id}`), [home]);
  const children = pages.childrenOf(page.id);

  async function enter(takeOver: boolean): Promise<void> {
    if (opening.current !== undefined || !mounted() || !canEditPageBody(pages, page.id)) {
      return;
    }
    const next = store.editPage(notebook, page.id);
    if (next === undefined) {
      return;
    }
    opening.current = next;
    setEntering(true);
    setRefusal(undefined);
    setRestored(undefined);
    roleRead.current = false;
    setIdleLeft(false);
    setToldLeft("");
    try {
      const opened = await next.begin(takeOver);
      if (!mounted() || !canEditPageBody(pages, page.id)) {
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
        roleTaken(error);
      }
    } finally {
      opening.current = undefined;
      if (mounted()) {
        setEntering(false);
      }
    }
  }

  // A refusal as forbidden says the account's role was taken away meanwhile: the notebooks are read again, and the
  // page follows its role (no Edit, the checkboxes disabled, the view not written again; M12/P2/S4, 6.2). Another
  // outcome, or a toggle starting, ends the wait for it; so does the read's answer (review B's C1-M1): the page
  // writable still, the role kept or the read failed, the focus is left; writable no more before it was refused (an
  // event took the role first: no render is to come), the focus, if lost, goes to the title at once; else the effect
  // below gives it, after the render the read brings. Writable or not is the store's as the answer comes, not a
  // render's.
  function roleTaken(why: unknown) {
    roleRead.current = why instanceof ApiError && why.code === "forbidden";
    if (!roleRead.current) {
      return;
    }
    const id = page.id;
    const writable = canEditPageBody(pages, id);
    void mutate(["notebooks", workspace.id])
      .catch(() => undefined)
      .finally(() => {
        if (!mounted() || !roleRead.current) {
          return;
        }
        if (canEditPageBody(pages, id)) {
          roleRead.current = false;
        } else if (!writable) {
          roleRead.current = false;
          if (focusLost()) {
            heading.current?.focus();
          }
        }
      });
  }

  // A task item's toggle refused (M5/P6 design 3.5): page.locked as Edit's refusal is, the note reading who holds
  // the lock and taking the focus from the checkbox, unless the focus has gone elsewhere meanwhile; another refusal
  // above the view. undefined, as a toggle starts, clears it.
  async function toggleRefused(error: unknown): Promise<void> {
    if (!(error instanceof ApiError && error.code === "page.locked")) {
      setRefusal(error);
      roleTaken(error);
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

  // The role read again after a refusal as forbidden, the page writable no more: the focus Edit had, gone, or a
  // checkbox had, disabled, goes to the page's title, as back from an edit without Edit (18.6; M12/P2/S4, 6.2, review
  // B's M1). A focus the reader put elsewhere meanwhile stays.
  useEffect(() => {
    if (editable || !roleRead.current) {
      return;
    }
    roleRead.current = false;
    if (focusLost()) {
      heading.current?.focus();
    }
  }, [editable, heading]);

  // The keys of the latest render: enter reads its page.
  const keys = useRef({ enter });
  useEffect(() => {
    keys.current = { enter };
  });
  const reading = editing === undefined;
  // Back from the edit, as the reading view shows: Edit takes the focus, or, without Edit (the account made a reader
  // meanwhile), the page's title; an Edit that shows later, a writer again, takes nothing (M12/P2/S1, 18.6).
  useEffect(() => {
    if (!reading || back.current === false) {
      return;
    }
    const focused = document.activeElement;
    if (back.current === true || focused === null || focused === document.body) {
      (editable ? edit : heading).current?.focus();
    }
    back.current = false;
  }, [reading, editable, heading]);
  useEffect(() => {
    if (!editable || !reading) {
      return undefined;
    }
    const mac = onMac();
    const onKeyDown = (event: KeyboardEvent) => {
      if (isMod(event, "e", mac) && !dialogOpen() && canEditPageBody(pages, page.id)) {
        event.preventDefault();
        // A key held down acts once: the edit just left is not entered again.
        if (!event.repeat) {
          void keys.current.enter(false);
        }
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [editable, reading, pages, page.id]);
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Breadcrumbs notebook={notebook} ancestors={pages.ancestorsOf(page.id)} page={page} href={href} />
        <div className="flex items-start justify-between gap-3">
          <h1
            ref={heading}
            tabIndex={-1}
            aria-describedby={restored === undefined ? undefined : restoredNote}
            className="min-w-0 text-3xl font-semibold break-words outline-none"
          >
            {page.name}
          </h1>
          {reading && (
            <div className="flex shrink-0 gap-2">
              {!gone && (
                <PageMenu
                  notebook={notebook}
                  page={page}
                  href={`${home}/settings/transfer`}
                  history={`${home}/history?page=${page.id}`}
                />
              )}
              {editable && (
                <Button
                  ref={edit}
                  variant="outline"
                  aria-busy={entering || undefined}
                  aria-describedby={idleLeft || toldLeft !== "" ? idleNote : undefined}
                  aria-disabled={entering || undefined}
                  onClick={() => void enter(false)}
                >
                  {t("page.edit")}
                </Button>
              )}
            </div>
          )}
        </div>
        {restored !== undefined && (
          <output id={restoredNote} className="block text-sm text-muted-foreground">
            {t("version.restored", { revision: restored })}
          </output>
        )}
      </div>
      <div className="space-y-6 xl:grid xl:grid-cols-[minmax(0,1fr)_15rem] xl:items-start xl:gap-8 xl:space-y-0">
        <div className="min-w-0 space-y-6">
          {editing === undefined ? (
            <>
              {refusal !== undefined && <Alert>{refusalText(refusal, t)}</Alert>}
              {(idleLeft || toldLeft !== "") && (
                <output id={idleNote} className="block text-sm whitespace-pre-line text-muted-foreground">
                  {[idleLeft ? t("page.idleLeft") : "", toldLeft].filter((said) => said !== "").join("\n")}
                </output>
              )}
              <div ref={lockNote} tabIndex={-1} className="outline-none">
                <EditLockNote
                  notebook={notebook}
                  page={page}
                  editHere={editable ? () => void enter(true) : undefined}
                  released={() => (edit.current ?? heading.current)?.focus()}
                />
              </div>
              {/* How a source is changed is a note, not an error: it is not announced each time the page opens (M12/P2/S1,
                  38.1). */}
              {source && (
                <p className="rounded-md border px-4 py-3 text-sm text-muted-foreground">{t("page.sourceReadonly")}</p>
              )}
              <AttachmentDrop notebook={notebook} parent={page.id} enabled={writer && !gone}>
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
                back.current = left.waited === true ? "fallen" : true;
                setIdleLeft(left.idle);
                setToldLeft(left.told ?? "");
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
          {!gone && <AttachmentsSection notebook={notebook} parent={page.id} editorsElsewhere={!reading} />}
        </div>
        <PagePanel notebook={notebook} page={page} editing={!reading} href={href} />
      </div>
    </div>
  );
});

/**
 * refusalText says a refusal shown above the view: a task item's toggle that would leave no task item there
 * (ticking "- [ ]: /u" makes a link reference definition, not_allowed on offset) says so (M12/P2/S2, 6.1); any
 * other refusal as the app says it.
 */
function refusalText(error: unknown, t: Translate): string | undefined {
  if (
    error instanceof ApiError &&
    error.code === "validation_failed" &&
    (error.problem?.errors ?? []).some((each) => each.field === "offset" && each.code === "not_allowed")
  ) {
    return t("page.taskWouldGo");
  }
  return errorText(error, t);
}
