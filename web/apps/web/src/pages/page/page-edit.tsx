import { observer } from "mobx-react-lite";
import { lazy, Suspense, useEffect, useRef } from "react";
import { useSWRConfig } from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { Loading } from "../../components/loading";
import { dialogOpen, isMod, onMac } from "../../app/shortcuts";
import type { SourceEditorHandle } from "../../editor/source-editor";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { useNewPageEditing, usePageTree } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { ConflictPanel } from "./conflict-panel";
import { PageEditingBar } from "./page-editing-bar";
import { UnsavedGuard } from "./unsaved-guard";

// The editor's chunk: loaded when a page is first edited (M4/P6 design 3.2).
const SourceEditor = lazy(() =>
  import("../../editor/source-editor").then((module) => ({ default: module.SourceEditor }))
);

/** composed runs act once the editor's input method composition ends, or now while there is no editor. */
function composed(editor: SourceEditorHandle | null, act: () => void) {
  if (editor === null) {
    act();
  } else {
    editor.whenComposed(act);
  }
}

type PageEditProps = {
  notebook: Notebook;
  page: TreeNode;
  /** done is called once the edit is over: saved, its session ended, the reading view read again. */
  done(): void;
};

/**
 * PageEdit is a page's edit mode (M4/P6 design 3.6, 3.7): the source
 * editor on the content read as it began, the bar with its status, Save
 * and Done. Mod+S saves; Mod+E and Done save what is unsaved, then leave
 * for the reading view, which is read again first. A key pressed while
 * the input method composes waits for the composition's end, so that
 * half a word is never saved; so does a button pressed. While the edit is
 * left, its content is held as it is: what was saved is what the reading
 * view shows. While the edit is unsaved, leaving the page asks first. A
 * save refused because the page changed shows the conflict above the
 * editor, with its heading focused; until the user keeps their text or
 * discards it, a save only brings the focus back there. A save that went
 * through leaves the reading view in SWR's cache to be read again.
 */
export const PageEdit = observer(function PageEdit({ notebook, page, done }: PageEditProps) {
  const { slug } = useWorkspace();
  const { mutate } = useSWRConfig();
  const pages = usePageTree(notebook);
  const editing = useNewPageEditing(page.id);
  const editor = useRef<SourceEditorHandle>(null);
  const conflictHeading = useRef<HTMLHeadingElement>(null);
  const { conflict } = editing;

  useEffect(() => {
    editing.start();
    return () => editing.end();
  }, [editing]);

  useEffect(() => {
    if (conflict !== undefined) {
      conflictHeading.current?.focus();
    }
  }, [conflict]);

  async function save(): Promise<boolean> {
    const current = editor.current;
    if (editing.conflict !== undefined) {
      conflictHeading.current?.focus();
      return false;
    }
    if (current === null) {
      return false;
    }
    const saved = await editing.save(current.text(), current.version());
    if (saved && editing.saved) {
      // The reading view cached is older than the page: it goes, and is read when shown, deduplication or not.
      await mutate(["page-view", page.id], undefined);
    }
    return saved;
  }

  async function keepMine(): Promise<void> {
    const current = editor.current;
    if (current === null) {
      return;
    }
    await editing.keepMine(current.text(), current.version());
    // A conflict again takes the focus to its heading; a save that failed otherwise leaves the panel gone.
    if (editing.conflict === undefined) {
      current.focus();
    }
  }

  function discardMine() {
    const theirs = editing.discardMine();
    if (theirs !== undefined) {
      editor.current?.load(theirs);
      editor.current?.focus();
    }
  }

  async function leave(): Promise<void> {
    const current = editor.current;
    // What is typed while the edit is left would not be saved: the content is held as it is.
    current?.hold(true);
    if (editing.unsaved && !(await save())) {
      current?.hold(false);
      if (editing.conflict === undefined) {
        current?.focus();
      }
      return;
    }
    editing.end();
    // The reading view, whose hook is not mounted while the editor is, is read into SWR's cache, deduplication or not.
    await mutate(["page-view", page.id], pages.view(page.id), { revalidate: false }).catch(() => undefined);
    done();
  }

  // The keys of the latest render: save and leave read its editing.
  const keys = useRef({ save, leave });
  useEffect(() => {
    keys.current = { save, leave };
  });
  useEffect(() => {
    const mac = onMac();
    const onKeyDown = (event: KeyboardEvent) => {
      const act = isMod(event, "s", mac) ? keys.current.save : isMod(event, "e", mac) ? keys.current.leave : undefined;
      if (act === undefined || dialogOpen()) {
        return;
      }
      event.preventDefault();
      composed(editor.current, () => void act());
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  if (editing.content === undefined) {
    return <NotLoaded error={editing.readFailure} retry={() => void editing.read()} />;
  }
  return (
    <div className="space-y-3">
      <PageEditingBar
        editing={editing}
        save={() => composed(editor.current, () => void save())}
        leave={() => composed(editor.current, () => void leave())}
      />
      {conflict !== undefined && (
        <ConflictPanel
          conflict={conflict}
          heading={conflictHeading}
          keep={() => composed(editor.current, () => void keepMine())}
          discard={discardMine}
        />
      )}
      <Suspense fallback={<Loading />}>
        <SourceEditor
          ref={editor}
          focusOnOpen
          content={editing.content.content}
          context={{ workspace: slug, notebook: notebook.id, page: page.id, role: notebook.role }}
          controls={{ save: async () => void (await save()), saving: () => editing.saving }}
          onChange={editing.changed}
        />
      </Suspense>
      <UnsavedGuard unsaved={editing.unsaved} stay={() => editor.current?.focus()} />
    </div>
  );
});
