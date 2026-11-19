import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type FormEvent, type RefObject } from "react";
import { useLocation, useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { useForm } from "../../app/form";
import type { HeldDialog } from "../../app/held-dialog";
import { useMounted } from "../../app/mounted";
import { canAddSource } from "../../app/source-permissions";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "../../components/ui/dialog";
import { Label } from "../../components/ui/label";
import { useT } from "../../i18n/i18n";
import { sourceTitleProblem } from "../../lib/source-title";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import { useAssets, usePageTree, useStore } from "../../stores/context";
import { UploadRows } from "../page/upload-rows";
import { useWorkspace } from "../workspace/workspace-layout";
import { duplicateText } from "./duplicate-text";
import { useNodeNavigate } from "../node-target";

type PendingPaste = { controller: AbortController; sent: boolean };

/** Closing cancels a paste still waiting in the tree queue; a submitted request keeps its server outcome. */
function cancelPending(pending: RefObject<PendingPaste | undefined>): void {
  if (pending.current !== undefined && !pending.current.sent) pending.current.controller.abort();
}

/** What the paste's form tells the dialog: whether something is entered, and whether it is being sent. */
type PasteState = { draft: boolean; sending: boolean };

/**
 * A source is created with its chosen title and body; file operations belong to the notebook's lasting queue. A paste
 * with a title or Markdown entered is not lost to Escape or a click outside: the dialog asks first, Keep editing
 * giving the focus back where it was; being sent, it stays, as closing would not stop what was sent; Cancel closes
 * as before (M12/P2/S5, 38.7).
 */
export function SourceAddDialog({
  notebook,
  parent,
  name,
  mode,
  held,
}: {
  notebook: Notebook;
  parent: string;
  name: string;
  mode: "paste" | "files";
  held: HeldDialog;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const navigate = useNavigate();
  const pages = usePageTree(notebook);
  const navigateNode = useNodeNavigate(pages);
  const location = useLocation();
  const mounted = useMounted();
  const opening = useRef<string | undefined>(undefined);
  const done = useRef(false);
  const pending = useRef<PendingPaste | undefined>(undefined);
  const paste = useRef<PasteState>({ draft: false, sending: false });
  const [asking, setAsking] = useState(false);
  // The element the focus was on as the dialog asked: Keep editing gives it back.
  const askedFrom = useRef<Element | null>(null);
  const keepButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (held.open && opening.current === undefined) opening.current = location.key;
  }, [held.open, location.key]);
  // Asked, the focus goes to the answer that loses nothing.
  useEffect(() => {
    if (asking) keepButton.current?.focus();
  }, [asking]);
  const changeOpen = (open: boolean) => {
    if (!open) {
      cancelPending(pending);
      setAsking(false);
    }
    held.onOpenChange(open);
  };
  // Escape or a click outside would close: a paste being sent stays, one with a draft asks first.
  const holding = (event: Event) => {
    if (mode !== "paste") return;
    if (paste.current.sending) {
      event.preventDefault();
    } else if (paste.current.draft) {
      event.preventDefault();
      askedFrom.current = document.activeElement;
      setAsking(true);
    }
  };
  const keepEditing = () => {
    setAsking(false);
    const from = askedFrom.current;
    askedFrom.current = null;
    if (from instanceof HTMLElement && from.isConnected) from.focus();
  };
  const openNode = (id: string) => {
    done.current = true;
    changeOpen(false);
    navigateNode(id);
  };
  return (
    <Dialog open={held.open} onOpenChange={changeOpen}>
      {held.open && (
        <DialogContent
          className="sm:max-w-xl"
          onEscapeKeyDown={(event) => {
            if (asking) {
              event.preventDefault();
              keepEditing();
            } else {
              holding(event);
            }
          }}
          onInteractOutside={(event) => {
            if (asking) {
              event.preventDefault();
            } else {
              holding(event);
            }
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            if (mounted() && opening.current === location.key) held.onClosed(done.current);
            opening.current = undefined;
            done.current = false;
          }}
        >
          <DialogTitle>{t(mode === "paste" ? "source.pasteTitle" : "source.filesTitle", { name })}</DialogTitle>
          <DialogDescription>{t(mode === "paste" ? "source.pasteBody" : "source.filesBody")}</DialogDescription>
          {asking && (
            <div role="alert" className="space-y-3 rounded-md border px-4 py-3 text-sm">
              <p>{t("source.discardAsk")}</p>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" ref={keepButton} onClick={keepEditing}>
                  {t("source.keepEditing")}
                </Button>
                <Button type="button" variant="destructive" onClick={() => changeOpen(false)}>
                  {t("source.discardClose")}
                </Button>
              </div>
            </div>
          )}
          {mode === "paste" ? (
            <PasteSource
              notebook={notebook}
              parent={parent}
              pending={pending}
              report={(state) => (paste.current = state)}
              openNode={openNode}
              cancel={() => changeOpen(false)}
              created={(id) => {
                done.current = true;
                changeOpen(false);
                void navigate(`/${slug}/notebooks/${notebook.id}/pages/${id}`, { state: arrived });
              }}
            />
          ) : (
            <SourceFiles notebook={notebook} parent={parent} close={() => changeOpen(false)} openNode={openNode} />
          )}
        </DialogContent>
      )}
    </Dialog>
  );
}

const titleMessages = {
  required: "field.required",
  too_long: "field.notebook_name.too_long",
  invalid_format: "field.notebook_name.invalid_format",
  not_allowed: "field.notebook_name.not_allowed",
} as const;

const PasteSource = observer(function PasteSource({
  notebook,
  parent,
  pending,
  report,
  openNode,
  cancel,
  created,
}: {
  notebook: Notebook;
  parent: string;
  pending: RefObject<PendingPaste | undefined>;
  /** Told whether a title or Markdown is entered, and whether it is being sent, as either changes. */
  report: (state: PasteState) => void;
  openNode: (id: string) => void;
  cancel: () => void;
  created: (id: string) => void;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const mounted = useMounted();
  const { key } = useLocation();
  const at = useRef(key);
  const busy = useRef(false);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const bodyId = useId();
  const { ref, sending, banner, problemOf, submit } = useForm(["title", "content"], {
    explain: (error) => duplicateText(error, pages, openNode),
    onField: { "page.title_taken": "title" },
    fieldTexts: {
      "title.too_long": titleMessages.too_long,
      "title.invalid_format": titleMessages.invalid_format,
      "title.not_allowed": titleMessages.not_allowed,
    },
  });
  useEffect(() => {
    at.current = key;
  }, [key]);
  useEffect(() => () => cancelPending(pending), [pending]);
  useEffect(() => {
    report({ draft: title !== "" || content !== "", sending });
  }, [report, title, content, sending]);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy.current) return;
    const problem = sourceTitleProblem(title);
    const from = at.current;
    busy.current = true;
    void submit(problem === undefined ? {} : { title: titleMessages[problem] }, async () => {
      const request: PendingPaste = { controller: new AbortController(), sent: false };
      pending.current = request;
      try {
        const page = await pages.createWithContent(parent, title.trim().normalize("NFC"), content, {
          signal: request.controller.signal,
          beforeSend: () => {
            if (!mounted() || at.current !== from) throw new DOMException("The paste was cancelled", "AbortError");
            if (!canAddSource(pages, parent)) throw new ApiError(403, { code: "forbidden" });
            request.sent = true;
          },
        });
        if (mounted() && at.current === from && !request.controller.signal.aborted) created(page.id);
      } catch (error) {
        // The address changed while the paste waited: nothing was sent, and no refusal to show.
        if (!(error instanceof DOMException && error.name === "AbortError")) throw error;
      } finally {
        if (pending.current === request) pending.current = undefined;
      }
    }).finally(() => (busy.current = false));
  }

  const bodyProblem = problemOf("content");
  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        name="title"
        label={t("page.title")}
        value={title}
        error={problemOf("title")}
        autoComplete="off"
        onChange={(event) => setTitle(event.target.value)}
      />
      <div className="space-y-2">
        <Label htmlFor={bodyId}>{t("source.markdown")}</Label>
        <textarea
          id={bodyId}
          name="content"
          value={content}
          rows={10}
          aria-invalid={bodyProblem !== undefined || undefined}
          aria-describedby={`${bodyId}-note`}
          className="w-full resize-y rounded-md border bg-transparent px-3 py-2 font-mono text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring aria-invalid:border-destructive"
          onChange={(event) => setContent(event.target.value)}
        />
        <p
          id={`${bodyId}-note`}
          className={bodyProblem === undefined ? "text-sm text-muted-foreground" : "text-sm text-destructive"}
        >
          {bodyProblem ?? t("source.newlineHint")}
        </p>
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={cancel}>
          {t("page.cancel")}
        </Button>
        <Button type="submit" disabled={sending || !canAddSource(pages, parent)}>
          {sending ? t("source.creating") : t("source.create")}
        </Button>
      </div>
    </form>
  );
});

const SourceFiles = observer(function SourceFiles({
  notebook,
  parent,
  close,
  openNode,
}: {
  notebook: Notebook;
  parent: string;
  close: () => void;
  openNode: (id: string) => void;
}) {
  const t = useT();
  const assets = useAssets(notebook);
  const pages = usePageTree(notebook);
  const { instance } = useStore();
  const picker = useRef<HTMLInputElement>(null);
  const choose = useRef<HTMLButtonElement>(null);
  const uploads = assets.uploads.filter((upload) => upload.parent === parent && !upload.fromEditor);
  return (
    <div className="space-y-4">
      <input
        ref={picker}
        type="file"
        multiple
        hidden
        aria-label={t("source.files")}
        onChange={(event) => {
          const files = [...(event.target.files ?? [])];
          event.target.value = "";
          if (canAddSource(pages, parent)) {
            assets.uploadSources(parent, files, {
              untitled: t("asset.untitled"),
              maxBytes: instance.info?.asset_max_bytes,
            });
          }
        }}
      />
      <Button
        ref={choose}
        variant="outline"
        disabled={!canAddSource(pages, parent)}
        onClick={() => picker.current?.click()}
      >
        {t("source.chooseFiles")}
      </Button>
      {uploads.length > 0 && (
        <UploadRows
          notebook={notebook}
          assets={assets}
          uploads={uploads}
          left={() => choose.current?.focus()}
          openNode={openNode}
        />
      )}
      <div className="flex justify-end">
        <Button type="button" variant="outline" onClick={close}>
          {t("source.close")}
        </Button>
      </div>
    </div>
  );
});
