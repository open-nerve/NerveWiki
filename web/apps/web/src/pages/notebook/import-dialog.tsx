import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type FormEvent, type ReactElement, type RefObject } from "react";
import useSWR, { useSWRConfig } from "swr";

import { useForm, type LocalProblems } from "../../app/form";
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
 * judging. The upload, which may take minutes, tells its progress and
 * stops; the dialog closed meanwhile asks first, and its upload stops as
 * the dialog goes. A refusal stays in the dialog; a connection cut, which
 * a refusal before the file is read may reach the browser as, says what
 * may have happened, and is not tried again. Started, the job goes first
 * in the notebook's jobs, read again then, and the focus goes to its row.
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
  const upload = useRef<AbortController | undefined>(undefined);
  const started = useRef(false);

  function close() {
    setStopping(false);
    setOpen(false);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (next) {
          setOpen(true);
        } else if (upload.current === undefined) {
          close();
        } else {
          setStopping(true);
        }
      }}
    >
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      {open && (
        <DialogContent
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
          {stopping && (
            <div role="alert" className="space-y-3 rounded-md border px-4 py-3 text-sm">
              <p>{t("transfer.importStopAsk")}</p>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => setStopping(false)}>
                  {t("transfer.importKeepGoing")}
                </Button>
                <Button
                  type="button"
                  variant="destructive"
                  onClick={() => {
                    upload.current?.abort();
                    close();
                  }}
                >
                  {t("transfer.importStop")}
                </Button>
              </div>
            </div>
          )}
          <ImportForm
            notebook={notebook}
            upload={upload}
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
  cancel: () => void;
  imported: (job: TransferJob) => void;
};

const ImportForm = observer(function ImportForm({ notebook, upload, cancel, imported }: ImportFormProps) {
  const transfers = useTransfers(notebook);
  const pages = usePageTree(notebook);
  const { instance, preferences } = useStore();
  const t = useT();
  const { mutate } = useSWRConfig();
  useSWR("instance", () => instance.load());
  useSWR(["pages", notebook.id], () => pages.load());
  const ids = { file: useId(), place: useId() };
  const [file, setFile] = useState<File | undefined>(undefined);
  const [place, setPlace] = useState(root);
  const [sent, setSent] = useState<{ sent: number; total: number } | undefined>(undefined);
  const { ref, sending, banner, problemOf, submit } = useForm(["parent_id", "file"], {
    texts: { "transfer.busy": "transfer.importBusy", "page.not_found": "transfer.importPlaceGone" },
    explain: (error) => (error instanceof TypeError ? t("transfer.importCut") : undefined),
  });
  // The upload stops as the dialog goes.
  useEffect(() => () => upload.current?.abort(), [upload]);
  const { locale } = preferences;
  const most = instance.info?.import_max_bytes;
  const tree = pages.tree;
  // A page at the deepest level holds nothing more: whatever went under it would be too deep.
  const parents = tree === undefined ? [] : [...tree.byId.values()].filter((each) => depthOf(tree, each.id) < maxDepth);
  // A place the tree, read again, no longer has is the root again: what the select shows goes out.
  const chosen = parents.some((each) => each.id === place) ? place : root;
  const notZip = file !== undefined && !/\.zip$/i.test(file.name);
  const fileProblem = problemOf("file");
  const placeProblem = problemOf("parent_id");

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const found: LocalProblems<"file"> =
      file === undefined
        ? { file: "field.required" }
        : most !== undefined && file.size > most
          ? { file: "field.file.too_long" }
          : {};
    void submit(found, async () => {
      if (file === undefined) {
        return;
      }
      const stop = new AbortController();
      upload.current = stop;
      setSent({ sent: 0, total: file.size });
      try {
        const job = await transfers.startImport(chosen === root ? null : chosen, file, {
          progress: (done, total) => setSent({ sent: done, total }),
          signal: stop.signal,
        });
        // The jobs are read again: a list whose read failed is polled no more until it is read.
        void mutate(["transfer-jobs", notebook.id]);
        imported(job);
      } catch (error) {
        if (!stop.signal.aborted) {
          throw error;
        }
      } finally {
        upload.current = undefined;
        setSent(undefined);
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
          value={chosen}
          disabled={sending}
          aria-invalid={placeProblem !== undefined || undefined}
          aria-describedby={placeProblem === undefined ? undefined : `${ids.place}-note`}
          onChange={(event) => setPlace(event.target.value)}
        >
          <ParentOptions pages={pages} parents={parents} />
        </NativeSelect>
        <Problem id={`${ids.place}-note`} text={placeProblem} />
      </div>
      {sent !== undefined && (
        <div className="space-y-1">
          <progress
            className="w-full"
            max={sent.total}
            value={sent.sent}
            aria-label={t("transfer.importSent", {
              sent: formatBytes(sent.sent, locale),
              total: formatBytes(sent.total, locale),
            })}
          />
          <p className="text-sm text-muted-foreground" aria-hidden>
            {t("transfer.importSent", { sent: formatBytes(sent.sent, locale), total: formatBytes(sent.total, locale) })}
          </p>
        </div>
      )}
      <div className="flex justify-end gap-2">
        {sending ? (
          <Button type="button" variant="outline" onClick={() => upload.current?.abort()}>
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
