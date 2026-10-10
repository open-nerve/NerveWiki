import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type FormEvent, type ReactElement, type RefObject } from "react";
import { useBlocker } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { useForm, type LocalProblems } from "../../app/form";
import { NotLoaded } from "../../app/not-loaded";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "../../components/ui/dialog";
import { Input } from "../../components/ui/input";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { formatBytes } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TransferJob } from "../../services/transfer.service";
import { usePageTree, useStore, useTransfers } from "../../stores/context";
import { depthOf, maxDepth } from "../../stores/page-tree";
import { ParentOptions, Problem, rootOption as root } from "./parent-options";

type ImportDialogProps = {
  notebook: Notebook;
  trigger: ReactElement;
  /** Told the job as the import starts, before the dialog closes. */
  onStarted: (job: TransferJob) => void;
  /** Gives the focus where it goes once the import started: the job's row. */
  focusAfter: () => void;
};

/**
 * ImportDialog imports a zip into the notebook, at its root or under a
 * page (M7/P6 design 4.2): it says what becomes of the archive's files,
 * and how large one the server takes. Before sending it checks the file's
 * size, and an import of the notebook under way among the jobs held; a
 * file that does not look like a zip goes all the same, the server
 * judging; a page chosen that the tree, read again, no longer has is
 * chosen again. The upload, which may take minutes, tells its progress and
 * stops; the dialog closed, or the page left, meanwhile asks first, and
 * the upload stops as the dialog goes. A refusal stays in the dialog; a
 * connection cut, which a refusal before the file is read may reach the
 * browser as, says what may have happened, and is not tried again. Each
 * end reads the notebook's jobs again; started, the job goes first in
 * them, and the focus goes to its row.
 */
export const ImportDialog = observer(function ImportDialog({
  notebook,
  trigger,
  onStarted,
  focusAfter,
}: ImportDialogProps) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [uploading, setUploading] = useState(false);
  const upload = useRef<AbortController | undefined>(undefined);
  const started = useRef(false);
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) => uploading && currentLocation.pathname !== nextLocation.pathname
  );
  const leaving = blocker.state === "blocked";
  // The prompt asks only while the zip uploads: an upload ended meanwhile has nothing to stop.
  const asking = uploading && (stopping || leaving);
  const keepButton = useRef<HTMLButtonElement>(null);
  const stopButton = useRef<HTMLButtonElement>(null);
  // Asked, the focus goes to the answer that loses nothing.
  useEffect(() => {
    if (asking) {
      keepButton.current?.focus();
    }
  }, [asking]);
  useEffect(() => {
    if (blocker.state === "blocked" && !uploading) {
      blocker.proceed();
    }
  }, [blocker, uploading]);

  function close() {
    setStopping(false);
    setOpen(false);
  }

  // Kept on, the focus goes back to the button that stops the upload, the question's going.
  function keep() {
    setStopping(false);
    blocker.reset?.();
    stopButton.current?.focus();
  }

  // An upload ended has nothing to ask about: the next does not find the question asked.
  function uploadingNow(on: boolean) {
    setUploading(on);
    if (!on) {
      setStopping(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (next) {
          setOpen(true);
        } else if (!uploading) {
          close();
        } else {
          setStopping(true);
        }
      }}
    >
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      {open && (
        <DialogContent
          onEscapeKeyDown={(event) => {
            if (asking) {
              event.preventDefault();
              keep();
            }
          }}
          onCloseAutoFocus={(event) => {
            if (started.current) {
              event.preventDefault();
              focusAfter();
            }
            started.current = false;
          }}
        >
          <DialogTitle>{t("transfer.importTitleOf", { name: notebook.name })}</DialogTitle>
          <DialogDescription>{t("transfer.importExplain")}</DialogDescription>
          {asking && (
            <div role="alert" className="space-y-3 rounded-md border px-4 py-3 text-sm">
              <p>{t("transfer.importStopAsk")}</p>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" ref={keepButton} onClick={keep}>
                  {t("transfer.importKeepGoing")}
                </Button>
                <Button
                  type="button"
                  variant="destructive"
                  onClick={() => {
                    upload.current?.abort();
                    if (leaving) {
                      blocker.proceed?.();
                    } else {
                      close();
                    }
                  }}
                >
                  {t(leaving ? "transfer.importStopLeave" : "transfer.importStopClose")}
                </Button>
              </div>
            </div>
          )}
          <ImportForm
            notebook={notebook}
            upload={upload}
            stopButton={stopButton}
            onUploading={uploadingNow}
            cancel={close}
            imported={(job) => {
              started.current = true;
              onStarted(job);
              close();
            }}
          />
        </DialogContent>
      )}
    </Dialog>
  );
});

type ImportFormProps = {
  notebook: Notebook;
  /** The upload going, which the dialog stops as it closes. */
  upload: RefObject<AbortController | undefined>;
  /** The button that stops the upload, which the dialog focuses as the upload goes on. */
  stopButton: RefObject<HTMLButtonElement | null>;
  /** Told as the upload starts and ends. */
  onUploading: (on: boolean) => void;
  cancel: () => void;
  imported: (job: TransferJob) => void;
};

const ImportForm = observer(function ImportForm({
  notebook,
  upload,
  stopButton,
  onUploading,
  cancel,
  imported,
}: ImportFormProps) {
  const transfers = useTransfers(notebook);
  const pages = usePageTree(notebook);
  const { instance, preferences } = useStore();
  const t = useT();
  const { mutate } = useSWRConfig();
  useSWR("instance", () => instance.load());
  const read = useSWR(["pages", notebook.id], () => pages.load());
  const ids = { file: useId(), place: useId() };
  const [file, setFile] = useState<File | undefined>(undefined);
  const [place, setPlace] = useState(root);
  const progress = useRef<((sent: number, total: number) => void) | undefined>(undefined);
  /** Whether the form is still shown: one gone, its generation's SWR cache may be gone too. */
  const shown = useRef(true);
  const { ref, sending, banner, problemOf, submit } = useForm(["parent_id", "file"], {
    texts: {
      "transfer.busy": "transfer.importBusy",
      "page.not_found": "transfer.importPlaceGone",
      server_busy: "transfer.queueFull",
      payload_too_large: "transfer.importTooLarge",
      bad_request: "transfer.importBadRequest",
    },
    explain: (error) => (error instanceof TypeError ? t("transfer.importCut") : undefined),
  });
  // The upload stops as the dialog goes.
  useEffect(() => {
    shown.current = true;
    return () => {
      shown.current = false;
      upload.current?.abort();
    };
  }, [upload]);
  // Sending, the import button is disabled: the focus goes to the button that stops it.
  useEffect(() => {
    if (sending) {
      stopButton.current?.focus();
    }
  }, [sending, stopButton]);
  const { locale } = preferences;
  const most = instance.info?.import_max_bytes;
  const tree = pages.tree;
  // A page at the deepest level holds nothing more: whatever went under it would be too deep.
  const parents = tree === undefined ? [] : [...tree.byId.values()].filter((each) => depthOf(tree, each.id) < maxDepth);
  // A place the tree, read again, no longer offers (deleted, or moved too deep) is not sent: the select says to choose
  // again, so that any place picked, the top level too, changes it.
  const gone = place !== root && !parents.some((each) => each.id === place);
  const notZip = file !== undefined && !/\.zip$/i.test(file.name);
  const fileProblem = problemOf("file");
  const placeProblem = problemOf("parent_id");

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const found: LocalProblems<"file" | "parent_id"> = {
      ...(file === undefined
        ? { file: "field.required" }
        : most !== undefined && file.size > most
          ? { file: "field.file.too_long" }
          : {}),
      ...(gone ? { parent_id: "field.place.gone" } : {}),
    };
    void submit(found, async () => {
      if (file === undefined) {
        return;
      }
      const stop = new AbortController();
      upload.current = stop;
      onUploading(true);
      try {
        const job = await transfers.startImport(place === root ? null : place, file, {
          progress: (done, total) => progress.current?.(done, total),
          signal: stop.signal,
        });
        imported(job);
      } catch (error) {
        if (!stop.signal.aborted) {
          throw error;
        }
      } finally {
        // An upload stopped may end after the next began: only its own is forgotten.
        if (upload.current === stop) {
          upload.current = undefined;
          onUploading(false);
        }
        // The jobs are read again at each end, the form still shown: a refusal may be another's import, and a list whose
        // read failed is polled no more until it is read.
        if (shown.current) {
          void mutate(["transfer-jobs", notebook.id]);
        }
      }
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {most !== undefined && (
        <p className="text-sm text-muted-foreground">{t("transfer.importMost", { size: formatBytes(most, locale) })}</p>
      )}
      {transfers.importing && <Alert>{t("transfer.importUnderWay")}</Alert>}
      {banner !== undefined && <Alert>{banner}</Alert>}
      <div className="space-y-2">
        <Label htmlFor={ids.file}>{t("transfer.importFile")}</Label>
        <Input
          id={ids.file}
          name="file"
          type="file"
          accept=".zip,application/zip"
          disabled={sending}
          aria-invalid={fileProblem !== undefined || undefined}
          aria-describedby={
            [fileProblem === undefined ? "" : `${ids.file}-note`, notZip ? `${ids.file}-zip` : ""].join(" ").trim() ||
            undefined
          }
          onChange={(event) => setFile(event.target.files?.[0])}
        />
        <Problem id={`${ids.file}-note`} text={fileProblem} />
        {notZip && (
          <p id={`${ids.file}-zip`} className="text-sm text-muted-foreground">
            {t("transfer.importNotZip")}
          </p>
        )}
      </div>
      <div className="space-y-2">
        <Label htmlFor={ids.place}>{t("transfer.importPlace")}</Label>
        <NativeSelect
          id={ids.place}
          name="parent_id"
          value={place}
          disabled={sending}
          aria-invalid={placeProblem !== undefined || undefined}
          aria-describedby={placeProblem === undefined ? undefined : `${ids.place}-note`}
          onChange={(event) => setPlace(event.target.value)}
        >
          {gone && (
            <option value={place} disabled>
              {t("transfer.importPlaceChoose")}
            </option>
          )}
          <ParentOptions pages={pages} parents={parents} />
        </NativeSelect>
        <Problem id={`${ids.place}-note`} text={placeProblem} />
        {tree === undefined && read.error !== undefined && (
          <NotLoaded error={read.error} retry={() => void read.mutate()} />
        )}
      </div>
      {sending && file !== undefined && (
        <UploadProgress
          total={file.size}
          listen={(tell) => {
            progress.current = tell;
          }}
        />
      )}
      <div className="flex justify-end gap-2">
        {sending ? (
          <Button ref={stopButton} type="button" variant="outline" onClick={() => upload.current?.abort()}>
            {t("transfer.importStop")}
          </Button>
        ) : (
          <Button type="button" variant="outline" onClick={cancel}>
            {t("transfer.dialogCancel")}
          </Button>
        )}
        <Button type="submit" disabled={sending || transfers.importing}>
          {sending ? t("transfer.importing") : t("transfer.importConfirm")}
        </Button>
      </div>
    </form>
  );
});

type UploadProgressProps = {
  total: number;
  /** Given what tells the bytes sent, and nothing as it goes: the form does not render again at each. */
  listen: (tell: ((sent: number, total: number) => void) | undefined) => void;
};

function UploadProgress({ total, listen }: UploadProgressProps) {
  const { preferences } = useStore();
  const t = useT();
  const [sent, setSent] = useState({ sent: 0, total });
  useEffect(() => {
    listen((done, all) => setSent({ sent: done, total: all }));
    return () => listen(undefined);
  }, [listen]);
  const text = t("transfer.importSent", {
    sent: formatBytes(sent.sent, preferences.locale),
    total: formatBytes(sent.total, preferences.locale),
  });
  return (
    <div className="space-y-1">
      <progress className="w-full" max={sent.total} value={sent.sent} aria-label={text} />
      <p className="text-sm text-muted-foreground" aria-hidden>
        {text}
      </p>
    </div>
  );
}
