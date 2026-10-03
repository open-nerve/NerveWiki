import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { Navigate, useParams } from "react-router";
import useSWR from "swr";

import { arrived, useArrivalFocus } from "../../app/arrival";
import { writesPages } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { dialogOpen, isMod, onMac } from "../../app/shortcuts";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import { useNotebook } from "../notebook/notebook-layout";
import { NotFoundPage } from "../not-found";
import { useWorkspace } from "../workspace/workspace-layout";
import { Breadcrumbs } from "./breadcrumbs";
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
 * from the tree too, so that the tree read again refreshes them.
 */
export const PageLayout = observer(function PageLayout() {
  const notebook = useNotebook();
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const { pageId = "" } = useParams();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  if (pages.nodes === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const page = pages.byId(pageId);
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
 * PageShell is the page found: where it is, its title, its reading view
 * and its children. A writer edits it in its place (M4/P6 design 3.7): by
 * Edit or Mod+E, which leave the reading view for the editor, the focus on
 * the page's title until the editor takes it; back from the edit, the
 * focus is on Edit. The edit is not in the address: a reload shows the
 * reading view.
 */
const PageShell = observer(function PageShell({ notebook, page }: { notebook: Notebook; page: TreeNode }) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const [editing, setEditing] = useState(false);
  const edit = useRef<HTMLButtonElement>(null);
  const back = useRef(false);
  const writer = writesPages(notebook.role);
  const home = `/${slug}/notebooks/${notebook.id}`;
  const href = (id?: string) => (id === undefined ? home : `${home}/pages/${id}`);
  const children = pages.childrenOf(page.id);
  useEffect(() => {
    if (!writer || editing) {
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
          heading.current?.focus();
          setEditing(true);
        }
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [writer, editing, heading]);
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Breadcrumbs notebook={notebook} ancestors={pages.ancestorsOf(page.id)} page={page} href={href} />
        <div className="flex items-start justify-between gap-3">
          <h1 ref={heading} tabIndex={-1} className="text-3xl font-semibold break-words outline-none">
            {page.name}
          </h1>
          {writer && !editing && (
            <Button
              ref={edit}
              variant="outline"
              onClick={() => {
                heading.current?.focus();
                setEditing(true);
              }}
            >
              {t("page.edit")}
            </Button>
          )}
        </div>
      </div>
      {editing ? (
        <PageEdit
          notebook={notebook}
          page={page}
          done={() => {
            back.current = true;
            setEditing(false);
          }}
        />
      ) : (
        <ReadingView notebook={notebook} page={page} />
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
