import { reaction } from "mobx";
import { observer } from "mobx-react-lite";
import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { useSWRConfig } from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { dialogOpen, isMod, onMac } from "../../app/shortcuts";
import { Loading } from "../../components/loading";
import { Button } from "../../components/ui/button";
import type { SourceEditorHandle } from "../../editor/source-editor";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import type { PageEditing } from "../../stores/page-editing";
import { useWorkspace } from "../workspace/workspace-layout";
import { ConflictPanel } from "./conflict-panel";
import { EditLostBanner } from "./edit-lost-banner";
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
  /** The edit, its session open: the view keeps it while mounted. */
  editing: PageEditing;
  /** done is called once the edit is over: saved, its session ended, the reading view read again. */
  done(): void;
};

/**
 * PageEdit is a page's edit mode (M4/P6 design 3.6, 3.7): the source
 * editor on the content read as it began, the bar with its status, Save
 * and Done. Mod+S saves; Mod+E and Done save what is unsaved, then leave
 * for the reading view, which is read again first. A key pressed while
 * the input method composes waits for the composition's end, so that
 * half a word is never saved; so does a button pressed. The edit is left
 * once at a time, its content held as it is meanwhile: it leaves only
 * with nothing unsaved, so that what was typed is never lost. While the edit is unsaved, leaving the page asks first. A
 * save refused because the page changed shows the conflict above the
 * editor, with its heading focused; until the user keeps their text or
 * discards it, a save only brings the focus back there. A save that went
 * through leaves the reading view in SWR's cache to be read again. A
 * content that cannot be read offers to try again, or Done to go back.
 *
 * An edit whose session is lost (M5/P4 design 3.8) saves no more: the
 * editor is read-only, through the registered extension the controls tell,
 * and a banner says why in place of the bar; Mod+S does nothing, and
 * Mod+E is Back to reading, which asks first while changes are not saved.
 */
export const PageEdit = observer(function PageEdit({ notebook, page, editing, done }: PageEditProps) {
  const { slug } = useWorkspace();
  const t = useT();
  const { mutate } = useSWRConfig();
  const pages = usePageTree(notebook);
  const editor = useRef<SourceEditorHandle>(null);
  const conflictHeading = useRef<HTMLHeadingElement>(null);
  const banner = useRef<HTMLDivElement>(null);
  const leaving = useRef(false);
  const [asking, setAsking] = useState(false);
  const mounted = useMounted();
  const { conflict } = editing;
  const { lost } = editing.session;

  useEffect(() => {
    editing.keep();
    return () => editing.letGo();
  }, [editing]);

  useEffect(() => {
    if (conflict !== undefined) {
      conflictHeading.current?.focus();
    }
  }, [conflict]);

  useEffect(() => {
    if (lost !== undefined) {
      banner.current?.focus();
    }
  }, [lost]);

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
      await mutate(["page-view", notebook.id, page.id], undefined);
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
    if (leaving.current) {
      return;
    }
    leaving.current = true;
    // What is typed while the edit is left would not be saved: the content is held as it is.
    current?.hold(true);
    const saved = !editing.unsaved || (await save());
    if (!mounted()) {
      return;
    }
    if (!saved || editing.unsaved) {
      leaving.current = false;
      current?.hold(false);
      if (editing.conflict === undefined) {
        current?.focus();
      }
      return;
    }
    await finish();
  }

  /**
   * finish ends the edit, its session's end answered, so that the lock read
   * next is no longer its; the lock read before the edit, the account's own
   * elsewhere when it took the edit over, goes from SWR's cache, to be read
   * as the note shows. The reading view, whose hook is not mounted while
   * the editor is, is read into SWR's cache, deduplication or not, unless
   * the page is gone from the tree: there is no view to read.
   */
  async function finish(): Promise<void> {
    await editing.end();
    await mutate(["edit-lock", page.id], undefined, { revalidate: false });
    if (pages.byId(page.id) !== undefined) {
      await mutate(["page-view", notebook.id, page.id], pages.view(page.id), { revalidate: false }).catch(
        () => undefined
      );
    }
    done();
  }

  /** backToReading leaves an edit whose session is lost, unsaved once confirmed. */
  function backToReading() {
    if (editing.unsaved) {
      setAsking(true);
    } else if (!leaving.current) {
      leaving.current = true;
      void finish();
    }
  }

  // The keys of the latest render: save and leave read its editing; an edit lost goes back to reading.
  const keys = useRef({ save, leave });
  useEffect(() => {
    keys.current = { save, leave: lost === undefined ? leave : () => Promise.resolve(backToReading()) };
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
    return (
      <div className="space-y-3">
        <NotLoaded error={editing.readFailure} retry={() => void editing.read()} />
        {editing.readFailure !== undefined && (
          <Button variant="outline" onClick={() => void leave()}>
            {t("editor.done")}
          </Button>
        )}
      </div>
    );
  }
  return (
    <div className="space-y-3">
      {lost === undefined ? (
        <PageEditingBar
          editing={editing}
          save={() => composed(editor.current, () => void save())}
          leave={() => composed(editor.current, () => void leave())}
        />
      ) : (
        <EditLostBanner ref={banner} lost={lost} unsaved={editing.unsaved} back={backToReading} />
      )}
      <ConfirmDialog
        // Stayed, the focus goes back to the banner, which Back to reading is in.
        held={{ open: asking, onOpenChange: setAsking, onClosed: (left) => void (left || banner.current?.focus()) }}
        title={t("editor.leaveTitle")}
        description={t("editor.leaveDescription")}
        confirmLabel={t("editor.leave")}
        sendingLabel={t("editor.leave")}
        cancelLabel={t("editor.stay")}
        confirm={finish}
      />
      {conflict !== undefined && lost === undefined && (
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
          controls={{
            save: async () => void (await save()),
            saving: () => editing.saving,
            session: () => ({ lost: editing.session.lost !== undefined }),
            onSessionChange: (listener) =>
              reaction(
                () => editing.session.lost,
                () => listener()
              ),
          }}
          onChange={editing.changed}
        />
      </Suspense>
      <UnsavedGuard
        unsaved={editing.unsaved}
        stay={() =>
          (editing.session.lost !== undefined
            ? banner.current
            : editing.conflict === undefined
              ? editor.current
              : conflictHeading.current
          )?.focus()
        }
      />
    </div>
  );
});
