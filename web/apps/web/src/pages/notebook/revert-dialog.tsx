import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type FormEvent, type RefObject } from "react";
import { Link } from "react-router";
import { useSWRConfig } from "swr";

import { useForm, type LocalProblems } from "../../app/form";
import { sourceHistoryText } from "./duplicate-text";
import { useNodeNavigate } from "../node-target";
import { lockedText } from "../../app/problem-messages";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "../../components/ui/alert-dialog";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { shortId, twinsOf } from "../../lib/twins";
import { ApiError, lost } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { ChangesetDetail } from "../../services/page.service";
import { usePageTree, useStore, useTransfers } from "../../stores/context";
import type { Press } from "../../stores/transfer.store";
import { openOf } from "../open-node";
import { watchReader } from "../page/readers-input";
import { useWorkspace } from "../workspace/workspace-layout";
import { countText, reasonText } from "./changeset-text";
import { distinctName } from "./distinct-name";

/** The most bytes a revert's message holds: the server's (domain.MaxMessageBytes). */
const messageBytes = 4096;

/**
 * Reverted is what a revert came to that its dialog closed for, which the
 * changeset's page says: undone, by the changeset that did it; nothing to
 * undo; refused, the changeset trimmed or out of reach; or its answer lost
 * (a 5xx, the network), its outcome to find out: the newest revert of the
 * changeset known as it was sent is before. A revert in the background
 * came to its job, started; or, its answer lost, to the job of its press,
 * if any, to find out (M8/P4 design 4.2).
 */
export type Reverted =
  | { kind: "reverted"; revert: string }
  | { kind: "unchanged" }
  | { kind: "refused"; error: unknown }
  | { kind: "unknown"; before: string | undefined }
  | { kind: "started"; job: string }
  | { kind: "startLost"; press: Press };

/**
 * RevertDialog is a changeset's Undo change (M8 design 4.8; M8/P3 design
 * 4.4): a dialog that says what the revert does, warns that links written
 * to new names and places stay where the changeset renamed or moved a node,
 * and takes an optional message, then, confirmed, reverts the changeset
 * through the page tree store's queue: once, however often Undo is
 * pressed, and nothing closes the dialog meanwhile. Its conflicts replace
 * what it says, each node with why and what to do, their title taking the
 * focus; Close gives it back to Undo change. A page someone edits, a
 * changeset too large, a server too busy and a message refused say so in
 * it. Anything else closes it, the changeset's page saying what came of it
 * (done), the focus to its title (heading) unless the reader did something
 * meanwhile (v0.1 design 13.2, item 26); sent is called as it sends.
 * Undo change, gone with the focus, gives it to the title as well. A
 * changeset too large to undo here asks instead whether to undo it in the
 * background (M8/P4 design 4.2), the message kept: its job started closes
 * the dialog as an undo done does; refused, it says why as an undo does.
 * An undo of the changeset out as it shows, sent before it did (the reader
 * went back and forward), it follows as its own (M5-polish 23.10): Undo
 * change busy, opening nothing, then what came of it said by the page,
 * any refusal there, the title taking the focus as it would; an answer
 * lost is found out by the newest undo its sender knew.
 */
export const RevertDialog = observer(function RevertDialog({
  notebook,
  changeset,
  heading,
  sent,
  done,
}: {
  notebook: Notebook;
  changeset: ChangesetDetail;
  heading: RefObject<HTMLElement | null>;
  sent: () => void;
  done: (reverted: Reverted) => void;
}) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [sending, setSending] = useState(false);
  const leaving = useRef(false);
  const pages = usePageTree(notebook);
  const navigateNode = useNodeNavigate(pages);
  const trigger = useRef<HTMLButtonElement>(null);
  useFocusLeaving(trigger, () => heading.current?.focus());
  // Whether the dialog closes for the title to take the focus.
  const toTitle = useRef(false);
  const out = pages.revertOut(changeset.id) !== undefined;
  // The page's latest: the effect below calls them once the undo it follows is answered.
  const page = useRef({ sent, done });
  useEffect(() => {
    page.current = { sent, done };
  });
  // An undo out as the page mounts is another mount's: followed as this one's (M5-polish 23.10). This mount's own
  // goes out later, through its dialog.
  useEffect(() => {
    const following = pages.revertOut(changeset.id);
    if (following === undefined) {
      return undefined;
    }
    let showing = true;
    page.current.sent();
    // Undo change pressed again, busy, is no act of the reader's.
    const reader = watchReader({ on: trigger.current });
    void (async () => {
      let reverted: Reverted;
      let focus = true;
      try {
        const answer = await following.answer;
        reverted =
          answer.changed && answer.changeset_id !== null
            ? { kind: "reverted", revert: answer.changeset_id }
            : { kind: "unchanged" };
        focus = answer.changed;
      } catch (error) {
        // Whatever the dialog would have kept in it is said by the page; Undo change shows it again, in the dialog.
        reverted = lost(error) ? { kind: "unknown", before: following.before } : { kind: "refused", error };
      }
      if (showing) {
        page.current.done(reverted);
        const at = document.activeElement;
        if (
          focus &&
          !reader.acted() &&
          (at === null || at === document.body || at === trigger.current || at === heading.current)
        ) {
          heading.current?.focus();
        }
      }
      reader.end();
    })();
    return () => {
      showing = false;
      reader.end();
    };
  }, [pages, changeset.id, heading]);
  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!sending && !(next && out)) {
          setOpen(next);
        }
      }}
    >
      {/* A small pill in the changeset's head's row, as a page's Edit is (M13/P5/S2, decision 929). */}
      <AlertDialogTrigger asChild>
        <Button
          ref={trigger}
          variant="outline"
          size="sm"
          shape="pill"
          aria-busy={out || undefined}
          aria-disabled={out || undefined}
        >
          {t(out ? "history.reverting" : "history.undo")}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent
        className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-xl"
        onCloseAutoFocus={(event) => {
          if (leaving.current) {
            event.preventDefault();
          } else if (toTitle.current) {
            event.preventDefault();
            heading.current?.focus();
          }
          toTitle.current = false;
          leaving.current = false;
        }}
      >
        <AlertDialogTitle>{t("history.revertTitle")}</AlertDialogTitle>
        <RevertForm
          notebook={notebook}
          changeset={changeset}
          sending={sending}
          setSending={setSending}
          sent={sent}
          openNode={(id) => {
            leaving.current = true;
            setOpen(false);
            navigateNode(id);
          }}
          close={(reverted, focus) => {
            toTitle.current = focus;
            setOpen(false);
            done(reverted);
          }}
        />
      </AlertDialogContent>
    </AlertDialog>
  );
});

/**
 * RevertForm is the dialog's content, mounted as it opens: what the revert
 * does, the message, and Undo; or, refused as too large, that it is, how
 * many nodes it touched, whether to undo it in the background, the
 * message, and Undo in the background, its title taking the focus; or,
 * refused for its conflicts, those.
 */
function RevertForm({
  notebook,
  changeset,
  sending,
  setSending,
  sent,
  close,
  openNode,
}: {
  notebook: Notebook;
  changeset: ChangesetDetail;
  sending: boolean;
  setSending: (sending: boolean) => void;
  sent: () => void;
  /** close closes the dialog for reverted, the title taking the focus when focus says so. */
  close: (reverted: Reverted, focus: boolean) => void;
  openNode: (id: string) => void;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const transfers = useTransfers(notebook);
  const workspace = useWorkspace();
  const { mutate } = useSWRConfig();
  const store = useStore();
  const { locale } = store.preferences;
  const me = store.account?.me?.id;
  const [message, setMessage] = useState("");
  const [conflict, setConflict] = useState<ApiError>();
  // Whether the undo was refused as too large to undo here: the dialog asks to undo it in the background.
  const [background, setBackground] = useState(false);
  // Whether the undo in the background was refused as too large for it too: no undo is offered (M5-polish 24.2).
  const [hopeless, setHopeless] = useState(false);
  // One undo a press: Undo stays where the focus is while it is out (aria-disabled), so it can be pressed again.
  const busy = useRef(false);
  const confirm = useRef<HTMLButtonElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const { ref, banner, problemOf, submit } = useForm(["message"], {
    explain: (error) => {
      const source = sourceHistoryText(error, pages, openNode);
      if (source !== undefined) return source;
      if (error instanceof ApiError) {
        switch (error.code) {
          // Past the background's bound: an undo here too large asks to undo it in the background instead.
          case "page.revert_too_large":
            return t("history.backgroundTooLarge");
          case "transfer.busy":
            return t("transfer.writingBusy");
          case "server_busy":
            return background ? t("transfer.queueFull") : undefined;
        }
      }
      return lockedText(error, t, me, (id) => distinctName(pages.tree, id, t));
    },
  });
  if (conflict !== undefined) {
    return <Conflicts notebook={notebook} changeset={changeset} refusal={conflict} />;
  }
  // What a revert may have changed: the changeset (but where the page reads it itself), its nodes, the history's
  // lists. SWR's mutate rejects only once the login's cache is gone, the page with it: nothing to say then (M8/P1
  // fix check 3, R3-1).
  const reread = (itself: boolean) => {
    if (itself) {
      void mutate(["changeset", notebook.id, changeset.id]).catch(() => undefined);
    }
    void mutate(["changeset-changes", notebook.id, changeset.id]).catch(() => undefined);
    void mutate((key) => Array.isArray(key) && key[0] === "changesets" && key[1] === notebook.id).catch(
      () => undefined
    );
  };

  async function revert(text: string | undefined): Promise<void> {
    // Where the reader was as they pressed it: the title takes the focus only if they did nothing meanwhile.
    const reader = watchReader({ on: confirm.current });
    const before = changeset.reverted_by[0];
    setSending(true);
    sent();
    try {
      const answer = await pages.revert(changeset.id, text, before);
      reread(true);
      close(
        answer.changed && answer.changeset_id !== null
          ? { kind: "reverted", revert: answer.changeset_id }
          : { kind: "unchanged" },
        answer.changed && !reader.acted()
      );
    } catch (error) {
      if (error instanceof ApiError && error.code === "page.revert_too_large") {
        setBackground(true);
        return;
      }
      if (!(await refused(error, reader))) {
        if (!lost(error)) {
          throw error;
        }
        // Undone or not: the page reads the changeset again to find out.
        reread(false);
        close({ kind: "unknown", before }, !reader.acted());
      }
    } finally {
      reader.end();
      setSending(false);
    }
  }

  // An undo in the background (M8/P4 design 4.2): its job started closes the dialog as an undo done does; its answer
  // lost, the page finds out from the jobs, its reverted_by being empty until the job's first batch (review B-M6).
  async function start(text: string | undefined): Promise<void> {
    const reader = watchReader({ on: confirm.current });
    const press = transfers.press();
    setSending(true);
    sent();
    try {
      const job = await transfers.startRevert(changeset.id, text);
      close({ kind: "started", job: job.id }, !reader.acted());
    } catch (error) {
      if (!(await refused(error, reader))) {
        if (!lost(error)) {
          // Too large for the background too, it never will start: only Cancel is left; the banner says why.
          if (error instanceof ApiError && error.code === "page.revert_too_large") {
            setHopeless(true);
          }
          throw error;
        }
        close({ kind: "startLost", press }, !reader.acted());
      }
    } finally {
      reader.end();
      setSending(false);
    }
  }

  // refused tells whether refusal is one the dialog leaves for: its conflicts, which replace what it says, the
  // jobs read first, a job that writes the tree under way perhaps what they came of (M8/P4A review A-M2); the
  // changeset trimmed, or out of reach, which close it as the page reads it again.
  async function refused(refusal: unknown, reader: ReturnType<typeof watchReader>): Promise<boolean> {
    if (refusal instanceof ApiError && refusal.code === "page.revert_conflict") {
      // Not read, the jobs held say what they can. The changeset is read again first: undone elsewhere meanwhile,
      // what the conflicts advise follows its reverted_by, not the one before (M12/P2/S2, 23.11; review A's N4); the
      // lists after it, not waited for. Its read failing, they advise as before.
      await transfers.load().catch(() => undefined);
      await mutate(["changeset", notebook.id, changeset.id]).catch(() => undefined);
      reread(false);
      setConflict(refusal);
      return true;
    }
    if (
      refusal instanceof ApiError &&
      (refusal.code === "page.changeset_trimmed" || refusal.code === "forbidden" || refusal.status === 404)
    ) {
      reread(true);
      if (refusal.code !== "page.changeset_trimmed") {
        // A role taken away meanwhile, or the notebook gone or out of reach: the pages follow the notebooks' roles.
        void mutate(["notebooks", workspace.id]).catch(() => undefined);
      }
      close({ kind: "refused", error: refusal }, !reader.acted());
      return true;
    }
    return false;
  }

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    // Only Cancel offered, Enter in the message still submits the form: it sends nothing (M5-polish 24.2).
    if (busy.current || hopeless) {
      return;
    }
    const text = message.trim();
    const bytes = new TextEncoder().encode(text).length;
    const found: LocalProblems<"message"> =
      bytes > messageBytes
        ? { message: "field.message.too_long" }
        : text.includes("\0")
          ? { message: "field.message.invalid_format" }
          : {};
    busy.current = true;
    const send = background ? start : revert;
    void submit(found, () => send(text === "" ? undefined : text)).finally(() => {
      busy.current = false;
    });
  }

  const nodes = countText(changeset.node_count, t, locale);
  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {/* All it says describes the dialog, read with its name: what the undo does, or that it is too large and the
          question, and that links written since stay (M12/P2/S1, 24.3, 49). */}
      <AlertDialogDescription asChild>
        <div className="space-y-4">
          {background ? (
            <TooLarge nodes={nodes} />
          ) : (
            <p>{t("history.revertBody", { nodes, contents: countText(changeset.content_count, t, locale) })}</p>
          )}
          {(changeset.renames || changeset.moves) && <p className="text-foreground">{t("history.revertLinks")}</p>}
        </div>
      </AlertDialogDescription>
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={t("history.revertMessage")}
        name="message"
        autoComplete="off"
        value={message}
        error={problemOf("message")}
        onChange={(event) => setMessage(event.target.value)}
      />
      {/* A line each, to the right, where one line holds not both: Cancel and Undo in the background at 320
          (decision 958, extended). */}
      <div className="flex flex-wrap justify-end gap-2">
        {/* Not disabled while it is out: a disabled button drops the focus (v0.1 design 13.2, item 12). The dialog
            does not close while it is out. */}
        <AlertDialogCancel asChild>
          <Button ref={cancel} variant="outline" aria-disabled={sending || undefined}>
            {t("page.cancel")}
          </Button>
        </AlertDialogCancel>
        {!hopeless && (
          <Confirm button={confirm} sending={sending} left={() => cancel.current?.focus()}>
            {background
              ? t(sending ? "history.backgroundStarting" : "history.backgroundConfirm")
              : t(sending ? "history.reverting" : "history.revertConfirm")}
          </Confirm>
        )}
      </div>
    </form>
  );
}

/**
 * Confirm is the dialog's Undo, or Undo in the background, busy while it
 * is out (sending). Gone with the focus, the undo too large for the
 * background too, it calls left before it leaves the document (M5-polish
 * 24.2): in a component of its own, so that it leaves with it.
 */
function Confirm({
  button,
  sending,
  left,
  children,
}: {
  button: RefObject<HTMLButtonElement | null>;
  sending: boolean;
  left: () => void;
  children: string;
}) {
  useFocusLeaving(button, left);
  return (
    <Button ref={button} type="submit" aria-busy={sending || undefined} aria-disabled={sending || undefined}>
      {children}
    </Button>
  );
}

/**
 * TooLarge says a changeset is too large to undo here, how many nodes it
 * touched, and asks whether to undo it in the background, which goes in
 * batches, what it undid kept when it stops on the way (M8 design section
 * 11, item 7): part of the dialog's description; its title takes the focus
 * as it shows, described by the question, which a screen reader does not
 * read again with the dialog's description as the focus moves in the open
 * dialog (M12/P2/S1, 24.3).
 */
function TooLarge({ nodes }: { nodes: string }) {
  const t = useT();
  const title = useRef<HTMLHeadingElement>(null);
  const ask = useId();
  useEffect(() => {
    title.current?.focus();
  }, []);
  return (
    <>
      <h3 ref={title} tabIndex={-1} aria-describedby={ask} className="font-medium text-foreground outline-none">
        {t("history.revertTooLarge", { nodes })}
      </h3>
      <p id={ask} className="text-foreground">
        {t("history.backgroundAsk")}
      </p>
    </>
  );
}

/**
 * Conflicts are what keep a revert from its changeset (M8/P3 design 4.4):
 * how many nodes, the first 100 listed, which their title, the dialog's
 * description, says as it takes the focus; each node by its name, why, and
 * what to do: its history, where it changed since; the trash, where the
 * page it was under went; the node put there since, to move it away; the
 * one with its name, to rename it, and, the node not in the tree (in the
 * trash, a deletion undone), its trash item, which a restore numbers. A
 * changeset that renamed a node can be renamed back by hand, too. Nodes
 * of one name are told apart by the end of their ids (v0.1 design 13.2,
 * item 17; M8/P3B review A-M5). A job that writes the notebook's tree
 * under way among the jobs, the nodes it wrote meanwhile are in the way,
 * changed: said before them (M8/P4A review A-M2).
 */
const Conflicts = observer(function Conflicts({
  notebook,
  changeset,
  refusal,
}: {
  notebook: Notebook;
  changeset: ChangesetDetail;
  refusal: ApiError;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const transfers = useTransfers(notebook);
  const { locale } = useStore().preferences;
  const title = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    title.current?.focus();
  }, []);
  const conflicts = refusal.problem?.conflicts ?? [];
  const count = refusal.problem?.conflict_count ?? conflicts.length;
  const home = `/${slug}/notebooks/${notebook.id}`;
  const found = (id: string) => pages.nodes?.find((node) => node.id === id);
  const opening = (id: string) => {
    const node = found(id);
    return node === undefined ? undefined : openOf({ id, kind: node.kind, parent: node.parent_id }, home);
  };
  const number = (n: number) => new Intl.NumberFormat(locale).format(n);
  const twins = twinsOf(
    conflicts.map((conflict) => ({ id: conflict.node_id, name: conflict.name })),
    (node) => node.name
  );
  return (
    <div className="space-y-4">
      <AlertDialogDescription asChild>
        <h3 ref={title} tabIndex={-1} className="font-medium text-foreground outline-none">
          {t("history.conflictsTitle", { count: number(count) })}
        </h3>
      </AlertDialogDescription>
      {transfers.writing && <p className="text-sm">{t("history.conflictsWriting")}</p>}
      {count > conflicts.length && (
        <p className="text-sm">{t("history.conflictsFirst", { count: number(conflicts.length) })}</p>
      )}
      <ul aria-label={t("history.conflicts")} className="space-y-3 text-sm">
        {conflicts.map((conflict) => {
          const holder = conflict.holder_id === undefined ? undefined : opening(conflict.holder_id);
          const node = opening(conflict.node_id);
          const twin = twins.has(conflict.node_id);
          const name = twin
            ? t("history.nodeTwin", { name: conflict.name, id: shortId(conflict.node_id) })
            : conflict.name;
          return (
            <li key={conflict.node_id} className="space-y-1">
              <p className="break-words">
                {reasonText(conflict, t)}
                {twin && (
                  <span className="text-muted-foreground"> {t("history.id", { id: shortId(conflict.node_id) })}</span>
                )}
              </p>
              <p className="flex flex-wrap gap-x-3 gap-y-1">
                {conflict.reason === "changed" && (
                  <Link
                    to={`${home}/history?page=${conflict.node_id}`}
                    aria-label={t("history.nodeHistoryLabel", { name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.nodeHistory")}
                  </Link>
                )}
                {conflict.reason === "parent_gone" && (
                  <Link
                    to={`${home}/trash`}
                    aria-label={t("history.conflictTrashLabel", { name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.conflictTrash")}
                  </Link>
                )}
                {conflict.reason === "children_added" && node !== undefined && (
                  <Link
                    to={node.to}
                    state={node.state}
                    aria-label={t("history.conflictOpenLabel", { name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.conflictOpen")}
                  </Link>
                )}
                {conflict.reason === "title_taken" && holder !== undefined && (
                  <Link
                    to={holder.to}
                    state={holder.state}
                    aria-label={t("history.conflictHolderLabel", { name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.conflictHolder")}
                  </Link>
                )}
                {conflict.reason === "title_taken" && pages.nodes !== undefined && node === undefined && (
                  <Link
                    to={`${home}/trash/${conflict.node_id}`}
                    aria-label={t("history.conflictRestoreLabel", { name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.conflictRestore")}
                  </Link>
                )}
              </p>
            </li>
          );
        })}
      </ul>
      {/* Undone elsewhere meanwhile, its page read again: renaming it back is no advice; undoing it again is not yet. */}
      {changeset.reverted_by.length > 0 ? (
        <p className="text-sm">{t("history.revertAgain")}</p>
      ) : (
        changeset.renames && <p className="text-sm">{t("history.conflictRenameBack")}</p>
      )}
      <div className="flex justify-end">
        <AlertDialogCancel asChild>
          <Button variant="outline">{t("history.close")}</Button>
        </AlertDialogCancel>
      </div>
    </div>
  );
});
