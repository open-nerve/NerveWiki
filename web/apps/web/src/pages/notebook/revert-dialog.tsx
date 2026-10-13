import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState, type FormEvent, type RefObject } from "react";
import { Link } from "react-router";
import { useSWRConfig } from "swr";

import { useForm, type LocalProblems } from "../../app/form";
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
import { useT, type Translate } from "../../i18n/i18n";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { ChangesetDetail, RevertConflict } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { watchReader } from "../page/readers-input";
import { useWorkspace } from "../workspace/workspace-layout";
import { countText } from "./changeset-text";
import { distinctName } from "./distinct-name";
import { openOf } from "./trash-row";

/** The most bytes a revert's message holds: the server's (domain.MaxMessageBytes). */
const messageBytes = 4096;

/**
 * Reverted is what a revert came to that its dialog closed for, which the
 * changeset's page says: undone, by the changeset that did it; nothing to
 * undo; refused, the changeset trimmed or out of reach; or its answer lost
 * (a 5xx, the network), its outcome to find out: the newest revert of the
 * changeset known as it was sent is before.
 */
export type Reverted =
  | { kind: "reverted"; revert: string }
  | { kind: "unchanged" }
  | { kind: "refused"; error: unknown }
  | { kind: "unknown"; before: string | undefined };

/**
 * lost tells whether a revert's answer says nothing of its outcome: a 5xx
 * but busy (the server's request deadline is a 500, its commit maybe
 * landed), or no answer at all; then the changeset is read again.
 */
function lost(error: unknown): boolean {
  return error instanceof TypeError || (error instanceof ApiError && error.status >= 500 && error.status !== 503);
}

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
 * Undo change, gone with the focus, gives it to the title as well.
 */
export function RevertDialog({
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
  const trigger = useRef<HTMLButtonElement>(null);
  useFocusLeaving(trigger, () => heading.current?.focus());
  // Whether the dialog closes for the title to take the focus.
  const toTitle = useRef(false);
  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!sending) {
          setOpen(next);
        }
      }}
    >
      <AlertDialogTrigger asChild>
        <Button ref={trigger} variant="outline">
          {t("history.undo")}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent
        className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-xl"
        onCloseAutoFocus={(event) => {
          if (toTitle.current) {
            event.preventDefault();
            heading.current?.focus();
          }
          toTitle.current = false;
        }}
      >
        <AlertDialogTitle>{t("history.revertTitle")}</AlertDialogTitle>
        <RevertForm
          notebook={notebook}
          changeset={changeset}
          sending={sending}
          setSending={setSending}
          sent={sent}
          close={(reverted, focus) => {
            toTitle.current = focus;
            setOpen(false);
            done(reverted);
          }}
        />
      </AlertDialogContent>
    </AlertDialog>
  );
}

/**
 * RevertForm is the dialog's content, mounted as it opens: what the revert
 * does, the message, and Undo; or, refused for its conflicts, those.
 */
function RevertForm({
  notebook,
  changeset,
  sending,
  setSending,
  sent,
  close,
}: {
  notebook: Notebook;
  changeset: ChangesetDetail;
  sending: boolean;
  setSending: (sending: boolean) => void;
  sent: () => void;
  /** close closes the dialog for reverted, the title taking the focus when focus says so. */
  close: (reverted: Reverted, focus: boolean) => void;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const workspace = useWorkspace();
  const { mutate } = useSWRConfig();
  const store = useStore();
  const { locale } = store.preferences;
  const me = store.account?.me?.id;
  const [message, setMessage] = useState("");
  const [conflict, setConflict] = useState<ApiError>();
  const busy = useRef(false);
  const confirm = useRef<HTMLButtonElement>(null);
  const { ref, banner, problemOf, submit } = useForm(["message"], {
    explain: (error) => {
      if (error instanceof ApiError && error.code === "page.revert_too_large") {
        return t("history.revertTooLarge", { nodes: countText(changeset.node_count, t, locale) });
      }
      return lockedText(error, t, me, (id) => distinctName(pages.tree, notebook, id, t));
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
      const answer = await pages.revert(changeset.id, text);
      reread(true);
      close(
        answer.changed && answer.changeset_id !== null
          ? { kind: "reverted", revert: answer.changeset_id }
          : { kind: "unchanged" },
        answer.changed && !reader.acted()
      );
    } catch (error) {
      if (error instanceof ApiError && error.code === "page.revert_conflict") {
        setConflict(error);
        return;
      }
      if (
        error instanceof ApiError &&
        (error.code === "page.changeset_trimmed" || error.code === "forbidden" || error.status === 404)
      ) {
        reread(true);
        if (error.code !== "page.changeset_trimmed") {
          // A role taken away meanwhile, or the notebook gone or out of reach: the pages follow the notebooks' roles.
          void mutate(["notebooks", workspace.id]).catch(() => undefined);
        }
        close({ kind: "refused", error }, !reader.acted());
        return;
      }
      if (lost(error)) {
        // Undone or not: the page reads the changeset again to find out.
        reread(false);
        close({ kind: "unknown", before }, !reader.acted());
        return;
      }
      throw error;
    } finally {
      reader.end();
      setSending(false);
    }
  }

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy.current) {
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
    void submit(found, () => revert(text === "" ? undefined : text)).finally(() => {
      busy.current = false;
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      <AlertDialogDescription>
        {t("history.revertBody", {
          nodes: countText(changeset.node_count, t, locale),
          contents: countText(changeset.content_count, t, locale),
        })}
      </AlertDialogDescription>
      {(changeset.renames || changeset.moves) && <p className="text-sm">{t("history.revertLinks")}</p>}
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={t("history.revertMessage")}
        name="message"
        autoComplete="off"
        value={message}
        error={problemOf("message")}
        onChange={(event) => setMessage(event.target.value)}
      />
      <div className="flex justify-end gap-2">
        <AlertDialogCancel asChild>
          <Button variant="outline" disabled={sending}>
            {t("page.cancel")}
          </Button>
        </AlertDialogCancel>
        <Button ref={confirm} type="submit" disabled={sending}>
          {sending ? t("history.reverting") : t("history.revertConfirm")}
        </Button>
      </div>
    </form>
  );
}

/** reasonText says why the node of conflict keeps the revert from its changeset. */
function reasonText(conflict: RevertConflict, t: Translate): string {
  const name = conflict.name;
  switch (conflict.reason) {
    case "changed":
      return t("history.conflict.changed", { name });
    case "expired":
      return t("history.conflict.expired", { name });
    case "parent_gone":
      return t("history.conflict.parent_gone", { name });
    case "children_added":
      return t("history.conflict.children_added", { name });
    case "title_taken":
      return t("history.conflict.title_taken", { name });
    case "cycle":
      return t("history.conflict.cycle", { name });
    case "too_deep":
      return t("history.conflict.too_deep", { name });
  }
}

/**
 * Conflicts are what keep a revert from its changeset (M8/P3 design 4.4):
 * how many nodes, the first 100 listed, which their title, the dialog's
 * description, says as it takes the focus; each node by its name, why, and
 * what to do: its history, where it changed since; the trash, where the
 * page it was under went; the node put there since, to move it away; the
 * one with its name, to rename it, and, the node not in the tree (in the
 * trash, a deletion undone), its trash item, which a restore numbers. A
 * changeset that renamed a node can be renamed back by hand, too.
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
  return (
    <div className="space-y-4">
      <AlertDialogDescription asChild>
        <h3 ref={title} tabIndex={-1} className="font-medium text-foreground outline-none">
          {t("history.conflictsTitle", { count: number(count) })}
        </h3>
      </AlertDialogDescription>
      {count > conflicts.length && (
        <p className="text-sm">{t("history.conflictsFirst", { count: number(conflicts.length) })}</p>
      )}
      <ul aria-label={t("history.conflicts")} className="space-y-3 text-sm">
        {conflicts.map((conflict) => {
          const holder = conflict.holder_id === undefined ? undefined : opening(conflict.holder_id);
          const node = opening(conflict.node_id);
          return (
            <li key={conflict.node_id} className="space-y-1">
              <p className="break-words">{reasonText(conflict, t)}</p>
              <p className="flex flex-wrap gap-x-3 gap-y-1">
                {conflict.reason === "changed" && (
                  <Link
                    to={`${home}/history?page=${conflict.node_id}`}
                    aria-label={t("history.nodeHistoryLabel", { name: conflict.name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.nodeHistory")}
                  </Link>
                )}
                {conflict.reason === "parent_gone" && (
                  <Link
                    to={`${home}/trash`}
                    aria-label={t("history.conflictTrashLabel", { name: conflict.name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.conflictTrash")}
                  </Link>
                )}
                {conflict.reason === "children_added" && node !== undefined && (
                  <Link
                    to={node.to}
                    state={node.state}
                    aria-label={t("history.conflictOpenLabel", { name: conflict.name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.conflictOpen")}
                  </Link>
                )}
                {conflict.reason === "title_taken" && holder !== undefined && (
                  <Link
                    to={holder.to}
                    state={holder.state}
                    aria-label={t("history.conflictHolderLabel", { name: conflict.name })}
                    className="underline underline-offset-4"
                  >
                    {t("history.conflictHolder")}
                  </Link>
                )}
                {conflict.reason === "title_taken" && pages.nodes !== undefined && node === undefined && (
                  <Link
                    to={`${home}/trash/${conflict.node_id}`}
                    aria-label={t("history.conflictRestoreLabel", { name: conflict.name })}
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
      {changeset.renames && <p className="text-sm">{t("history.conflictRenameBack")}</p>}
      <div className="flex justify-end">
        <AlertDialogCancel asChild>
          <Button variant="outline">{t("history.close")}</Button>
        </AlertDialogCancel>
      </div>
    </div>
  );
});
