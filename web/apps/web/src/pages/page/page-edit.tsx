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
import { PageEditingBar } from "./page-editing-bar";
import { UnsavedGuard } from "./unsaved-guard";

// The editor's chunk: loaded when a page is first edited (M4/P6 design 3.2).
const SourceEditor = lazy(() =>
  import("../../editor/source-editor").then((module) => ({ default: module.SourceEditor }))
);

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
 * half a word is never saved. While the edit is unsaved, leaving the
 * page asks first.
 */
export const PageEdit = observer(function PageEdit({ notebook, page, done }: PageEditProps) {
  const { slug } = useWorkspace();
  const { mutate } = useSWRConfig();
  const pages = usePageTree(notebook);
  const editing = useNewPageEditing(page.id);
  const editor = useRef<SourceEditorHandle>(null);

  useEffect(() => {
    editing.start();
    return () => editing.end();
  }, [editing]);

  async function save(): Promise<boolean> {
    const current = editor.current;
    if (current === null) {
      return false;
    }
    return editing.save(current.text(), current.version());
  }

  async function leave(): Promise<void> {
    if (editing.unsaved && !(await save())) {
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
      const run = () => void act();
      if (editor.current === null) {
        run();
      } else {
        editor.current.whenComposed(run);
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  if (editing.content === undefined) {
    return <NotLoaded error={editing.readFailure} retry={() => void editing.read()} />;
  }
  return (
    <div className="space-y-3">
      <PageEditingBar editing={editing} save={() => void save()} leave={() => void leave()} />
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
      {editing.unsaved && <UnsavedGuard />}
    </div>
  );
});
