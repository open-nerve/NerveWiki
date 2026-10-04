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
import { idleLimit } from "../../editor/idle-exit";
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

/**
 * composed runs act once the editor's input method composition ends, or
 * now while there is no editor; drop instead if the editor goes first.
 */
function composed(editor: SourceEditorHandle | null, act: () => void, drop?: () => void) {
  if (editor === null) {
    act();
  } else {
    editor.whenComposed(act, drop);
  }
}

/** Left is how an edit ended: idle, left for a long time without input. */
export type Left = { idle: boolean };

type PageEditProps = {
  notebook: Notebook;
  page: TreeNode;
  /** The edit, its session open: the view keeps it while mounted. */
  editing: PageEditing;
  /** done is called once the edit is over: saved, its session ended, the reading view read again. */
  done(left: Left): void;
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
  // Whether the user asked for the save last sent: a conflict it runs into takes the focus to its heading; one that
  // autosave or the idle exit runs into leaves the focus where it is, the status saying so (M5/P5 design 3.6).
  const asked = useRef(true);
  // How many saves the user asked for are out: a quiet save sent meanwhile does not speak for them.
  const askedOut = useRef(0);
  const [asking, setAsking] = useState(false);
  const mounted = useMounted();
  const { conflict } = editing;
  const { lost } = editing.session;
  const shown = editing.content !== undefined;

  useEffect(() => {
    editing.keep();
    return () => editing.letGo();
  }, [editing]);

  useEffect(() => {
    if (conflict !== undefined && asked.current) {
      conflictHeading.current?.focus();
    }
  }, [conflict]);

  // The banner takes the focus as the edit is lost, or as the content comes when it was lost before.
  useEffect(() => {
    if (lost !== undefined) {
      banner.current?.focus();
    }
  }, [lost, shown]);

  /** save saves the content; quietly, not asked by the user, it moves no focus. */
  async function save(quietly = false): Promise<boolean> {
    const current = editor.current;
    if (editing.conflict !== undefined) {
      if (!quietly) {
        conflictHeading.current?.focus();
      }
      return false;
    }
    if (current === null) {
      return false;
    }
    if (!quietly) {
      asked.current = true;
      askedOut.current += 1;
    } else if (askedOut.current === 0) {
      asked.current = false;
    }
    let saved: boolean;
    try {
      saved = await editing.save(current.text(), current.version());
    } finally {
      if (!quietly) {
        askedOut.current -= 1;
      }
    }
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
    asked.current = true;
    askedOut.current += 1;
    try {
      await editing.keepMine(current.text(), current.version());
    } finally {
      askedOut.current -= 1;
    }
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

  async function leave(left: Left = { idle: false }): Promise<void> {
    const current = editor.current;
    if (leaving.current) {
      return;
    }
    leaving.current = true;
    // What is typed while the edit is left would not be saved: the content is held as it is.
    current?.hold(true);
    const saved = !editing.unsaved || (await save(left.idle));
    if (!mounted()) {
      return;
    }
    if (!saved || editing.unsaved) {
      leaving.current = false;
      current?.hold(false);
      // Left by the user, the focus goes back to the editor; the idle exit moves none.
      if (!left.idle && editing.conflict === undefined) {
        current?.focus();
      }
      return;
    }
    await finish(left);
  }

  /**
   * finish ends the edit, its session's end answered, so that the lock read
   * next is no longer its; the lock read before the edit, the account's own
   * elsewhere when it took the edit over, goes from SWR's cache, to be read
   * as the note shows. The reading view, whose hook is not mounted while
   * the editor is, is read into SWR's cache, deduplication or not, unless
   * there is no view to read: the page is gone from the tree, or the edit
   * was lost as the page went or out of the account's reach (a notebook
   * gone keeps its tree as it was).
   */
  async function finish(left: Left = { idle: false }): Promise<void> {
    await editing.end();
    await mutate(["edit-lock", page.id], undefined, { revalidate: false });
    const reason = editing.session.lost?.reason;
    if (reason !== "gone" && reason !== "no_access" && pages.byId(page.id) !== undefined) {
      await mutate(["page-view", notebook.id, page.id], pages.view(page.id), { revalidate: false }).catch(
        () => undefined
      );
    }
    done(left);
  }

  /**
   * leaveIdle leaves an edit gone long without input (M5/P5 design 3.6),
   * as Done does once a composition ends, saying so on the reading view,
   * moving no focus while it stays: with a conflict open (its panel
   * decides) or a save that fails. It stays, and settles all the same,
   * while the session is lost (its banner stays), once a composition it
   * waited for ends (only the user ends one: that is input), or when the
   * editor goes first.
   */
  function leaveIdle(): Promise<void> {
    return new Promise((resolve) => {
      let waited = false;
      composed(
        editor.current,
        () => {
          if (waited || editing.session.lost !== undefined) {
            resolve();
          } else {
            void leave({ idle: true }).then(resolve, resolve);
          }
        },
        resolve
      );
      waited = true;
    });
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

  /**
   * signingOut saves what is unsaved for the sign-out, quietly: the content is
   * held as it is, as when the edit is left, since what is typed meanwhile
   * would not be saved before the edit ends (M4–M5 Codex review R1).
   */
  function signingOut(): Promise<boolean> {
    editor.current?.hold(true);
    return save(true);
  }

  // The keys of the latest render: save and leave read its editing; an edit lost goes back to reading.
  const keys = useRef({ save, leave, leaveIdle, signingOut });
  useEffect(() => {
    keys.current = {
      save,
      leave: lost === undefined ? leave : () => Promise.resolve(backToReading()),
      leaveIdle,
      signingOut,
    };
  });
  // The sign-out saves what is unsaved through the editor shown, before the edit ends.
  useEffect(() => {
    editing.savesThrough(() => keys.current.signingOut());
    return () => editing.savesThrough(undefined);
  }, [editing]);
  // An edit whose content is not read has no editor to time it: it is left as idle after as long all the same,
  // from the last try to read it.
  const unread = editing.readFailure;
  useEffect(() => {
    if (shown) {
      return undefined;
    }
    const idle = setTimeout(() => void keys.current.leaveIdle(), idleLimit);
    return () => clearTimeout(idle);
  }, [shown, unread]);
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
          // An edit lost before its editor is made leaves the focus on the banner.
          focusOnOpen={lost === undefined}
          content={editing.content.content}
          context={{ workspace: slug, notebook: notebook.id, page: page.id, role: notebook.role }}
          controls={{
            // Autosave's: quiet, a conflict open is its panel's, one run into moves no focus.
            save: async () => void (await save(true)),
            saving: () => editing.saving,
            session: () => ({ lost: editing.session.lost !== undefined }),
            onSessionChange: (listener) =>
              reaction(
                () => editing.session.lost,
                () => listener()
              ),
            leave: leaveIdle,
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
