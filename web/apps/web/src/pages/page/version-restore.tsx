import { useRef, useState } from "react";
import useSWR, { useSWRConfig } from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { errorText, lockedText } from "../../app/problem-messages";
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
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { PageVersionContent, TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { distinctName } from "../notebook/distinct-name";
import { Comparison } from "./comparison";

type VersionRestoreProps = {
  notebook: Notebook;
  page: TreeNode;
  version: PageVersionContent;
  /** said has the version's page say text, the dialog gone: nothing was restored. */
  said: (text: string) => void;
  /** restored takes the tab to the page, the version restored. */
  restored: () => void;
};

/**
 * VersionRestore is Restore this version (M8 design 4.4; P1 design 4.5):
 * a dialog that reads the page's content as it is now and shows what the
 * restore changes of it, from it to the version, then, confirmed, restores
 * the version on the revision read, through the page tree store's queue:
 * once, however often Restore is pressed, and nothing closes the dialog
 * meanwhile. Restored, the tab goes to the page (restored), which says
 * so, and the page's view and versions are read again; the focus is the
 * page's, not given back to the trigger. A version the page holds already
 * writes nothing, which the version's page says (said) as the dialog
 * closes, as it says a page gone (404). A page written since it was read
 * has its content read again, and the comparison with it, and the dialog
 * stays, saying so once that comparison shows: nothing is sent again by
 * itself (v0.1 design 13.2, item 7). A page someone edits says who, as a
 * lock does elsewhere, its lock's note read again; any other refusal stays
 * in the dialog. Restore sends only what shows: the comparison on screen
 * is with the content whose revision it sends (M8/P1 fix check 1, N1).
 */
export function VersionRestore({ notebook, page, version, said, restored }: VersionRestoreProps) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [sending, setSending] = useState(false);
  // Whether the restore went through: the focus is the page's then.
  const done = useRef(false);
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
        <Button variant="outline">{t("version.restore")}</Button>
      </AlertDialogTrigger>
      <AlertDialogContent
        className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-3xl"
        onCloseAutoFocus={(event) => {
          if (done.current) {
            event.preventDefault();
          }
        }}
      >
        <RestoreForm
          notebook={notebook}
          page={page}
          version={version}
          sending={sending}
          setSending={setSending}
          close={(text) => {
            said(text);
            setOpen(false);
          }}
          restored={() => {
            done.current = true;
            restored();
          }}
        />
      </AlertDialogContent>
    </AlertDialog>
  );
}

type RestoreFormProps = Omit<VersionRestoreProps, "said"> & {
  sending: boolean;
  setSending: (sending: boolean) => void;
  /** close closes the dialog, nothing restored, the version's page saying text. */
  close: (text: string) => void;
};

/** RestoreForm is the dialog's content, mounted as it opens: the page's content is read then, by the key it is compared by. */
function RestoreForm({ notebook, page, version, sending, setSending, close, restored }: RestoreFormProps) {
  const t = useT();
  const pages = usePageTree(notebook);
  const me = useStore().account?.me?.id;
  const { mutate } = useSWRConfig();
  const current = useSWR(["page-content", notebook.id, page.id], () => pages.content(page.id));
  const [failure, setFailure] = useState<unknown>(undefined);
  // The revision a restore was refused on, the page written since.
  const [refused, setRefused] = useState<number>();
  // The page's content whose comparison shows: none while one runs.
  const [shown, setShown] = useState<string>();
  const ready = current.data !== undefined && shown === current.data.content;

  async function restore(): Promise<void> {
    const read = current.data;
    if (read === undefined || sending) {
      return;
    }
    setSending(true);
    setFailure(undefined);
    setRefused(undefined);
    try {
      const answer = await pages.restoreVersion(page.id, version.revision, read.revision);
      if (!answer.changed) {
        close(t("version.unchanged"));
        return;
      }
      // The page's view is older than the page: it goes, read as the page shows; its versions have one more.
      // SWR's mutate rejects only once the login's cache is gone (signed out in another tab, say), the dialog with
      // it: then nothing is said (M8/P1 fix check 3, R3-1).
      void mutate(["page-view", notebook.id, page.id], undefined).catch(() => undefined);
      void mutate(["page-versions", notebook.id, page.id]).catch(() => undefined);
      restored();
    } catch (error) {
      if (error instanceof ApiError && error.code === "page.revision_mismatch") {
        setRefused(read.revision);
        // SWR answers what it holds, the content read before when the read fails. It rejects only once the login's
        // cache is gone (signed out in another tab, say), the dialog with it: nothing to say then (R3-1).
        const now = await current.mutate().catch(() => undefined);
        if (now?.revision === read.revision) {
          // Not read again: the refusal says why.
          setFailure(error);
        }
      } else if (error instanceof ApiError && error.status === 404) {
        close(error.code === "page.not_found" ? t("version.gone") : (errorText(error, t) ?? ""));
      } else {
        if (error instanceof ApiError && error.code === "page.locked") {
          // As above: it rejects only once the login's cache is gone, the dialog with it (fix check 4, R4-1).
          void mutate(["edit-lock", page.id]).catch(() => undefined);
        }
        setFailure(error);
      }
    } finally {
      setSending(false);
    }
  }

  const failed =
    failure === undefined
      ? undefined
      : (lockedText(failure, t, me, (id) => distinctName(pages.tree, notebook, id, t)) ?? errorText(failure, t));
  return (
    <>
      <AlertDialogTitle>{t("version.restoreTitle", { revision: version.revision })}</AlertDialogTitle>
      <AlertDialogDescription>{t("version.restoreBody")}</AlertDialogDescription>
      {ready && refused !== undefined && current.data?.revision !== refused && (
        <Alert>{t("version.changedMeanwhile")}</Alert>
      )}
      {current.data === undefined ? (
        <NotLoaded error={current.error} retry={() => void current.mutate()} />
      ) : (
        <Comparison
          before={current.data.content}
          after={version.content}
          label={t("version.currentDiff", { revision: version.revision })}
          beforeName={t("version.now")}
          afterName={t("version.title", { revision: version.revision })}
          shown={setShown}
        />
      )}
      {failed !== undefined && <Alert>{failed}</Alert>}
      <div className="flex justify-end gap-2">
        <AlertDialogCancel asChild>
          <Button variant="outline" disabled={sending}>
            {t("page.cancel")}
          </Button>
        </AlertDialogCancel>
        <Button disabled={sending || !ready} onClick={() => void restore()}>
          {sending ? t("version.restoring") : t("version.restoreConfirm")}
        </Button>
      </div>
    </>
  );
}
