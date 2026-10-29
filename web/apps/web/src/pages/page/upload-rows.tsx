import { runInAction } from "mobx";
import { observer } from "mobx-react-lite";
import { useRef, type FormEvent, type ReactNode } from "react";

import { useFocusLeaving } from "../../lib/focus-leaving";
import { sourceTitleProblem, type SourceTitleProblem } from "../../lib/source-title";
import { useForm } from "../../app/form";
import { writesPages } from "../../app/effective-role";
import { errorText, formErrors, type FieldMessage } from "../../app/problem-messages";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { formatBytes } from "../../i18n/format";
import { useT, type Translate } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import { isRefusal, sourceMaxBytes, type AssetStore, type Upload } from "../../stores/asset.store";
import { usePageTree, useStore } from "../../stores/context";
import { duplicateText, useNodeNavigate } from "../notebook/duplicate-text";

/**
 * UploadRows are the uploads under a page or the root (M7/P4 design 3.5),
 * above its attachments: each going with its progress, a bar and its
 * percentage, and Cancel; each sent whole, or answered, finishing, no
 * longer to be cancelled; each failed with why, and Dismiss. One button
 * does each in turn, so that the focus on it stays as the upload goes on;
 * a row that leaves with the focus in it gives it to left.
 */
export const UploadRows = observer(function UploadRows({
  notebook,
  assets,
  uploads,
  left,
  openNode,
}: {
  notebook: Notebook;
  assets: AssetStore;
  uploads: readonly Upload[];
  left: () => void;
  openNode?: (id: string) => void;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const navigate = useNodeNavigate(pages);
  const explain = (error: unknown) => duplicateText(error, pages, openNode ?? navigate);
  const canRetry = () => {
    const role = pages.notebook?.role;
    return role !== undefined && writesPages(role);
  };
  return (
    <ul aria-label={t("asset.uploads")} className="divide-y rounded-md border text-sm">
      {uploads.map((upload) =>
        upload.target === "source-page" ? (
          <SourceUploadRow
            key={upload.key}
            assets={assets}
            upload={upload}
            left={left}
            explain={explain}
            canRetry={canRetry}
          />
        ) : (
          <UploadRow key={upload.key} assets={assets} upload={upload} left={left} explain={explain} />
        )
      )}
    </ul>
  );
});

const UploadRow = observer(function UploadRow({
  assets,
  upload,
  left,
  explain,
}: {
  assets: AssetStore;
  upload: Upload;
  left: () => void;
  explain: (error: unknown) => ReactNode | undefined;
}) {
  const t = useT();
  const { instance, preferences } = useStore();
  const row = useRef<HTMLLIElement>(null);
  // Before the row leaves the document: the focus in it goes elsewhere, not to the page's start.
  useFocusLeaving(row, left);
  const failed = upload.failure !== undefined;
  const finishing = !failed && (upload.answered || (upload.total > 0 && upload.sent >= upload.total));
  const percent = upload.total > 0 ? Math.floor((upload.sent / upload.total) * 100) : 0;
  return (
    <li ref={row} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2">
      <span className="min-w-0 flex-1 truncate">{upload.name}</span>
      <span className="flex items-center gap-3">
        {failed ? (
          <span role="alert" className="text-destructive">
            {explain(upload.failure) ??
              failureText(upload.failure, t, preferences.locale, instance.info?.asset_max_bytes)}
          </span>
        ) : (
          <>
            <progress
              value={upload.sent}
              max={Math.max(upload.total, 1)}
              aria-label={t("asset.progressOf", { name: upload.name })}
              className="h-2 w-24"
            />
            <span className="w-10 text-right text-xs text-muted-foreground tabular-nums">{`${percent}%`}</span>
          </>
        )}
      </span>
      <Button
        variant="ghost"
        aria-label={
          failed
            ? t("asset.dismissUpload", { name: upload.name })
            : finishing
              ? t("asset.finishingUpload", { name: upload.name })
              : t("asset.cancelUpload", { name: upload.name })
        }
        aria-disabled={finishing || undefined}
        onClick={() => {
          if (failed) {
            assets.dismiss(upload);
          } else if (!finishing) {
            upload.cancel();
          }
        }}
      >
        {failed ? t("asset.dismiss") : finishing ? t("asset.finishing") : t("page.cancel")}
      </Button>
    </li>
  );
});

const titleMessages = {
  required: "field.required",
  too_long: "field.notebook_name.too_long",
  invalid_format: "field.notebook_name.invalid_format",
  not_allowed: "field.notebook_name.not_allowed",
} as const satisfies Record<SourceTitleProblem, FieldMessage>;

const sourceFormTexts = {
  onField: { "page.title_taken": "title" },
  texts: { "page.title_taken": "source.uploadTitleTaken" },
  fieldTexts: {
    "title.too_long": titleMessages.too_long,
    "title.invalid_format": titleMessages.invalid_format,
    "title.not_allowed": titleMessages.not_allowed,
  },
} as const;

/** Source rows keep title drafts in the persistent Upload, including across closing and reopening a modal. */
const SourceUploadRow = observer(function SourceUploadRow({
  assets,
  upload,
  left,
  explain,
  canRetry,
}: {
  assets: AssetStore;
  upload: Upload;
  left: () => void;
  explain: (error: unknown) => ReactNode | undefined;
  canRetry: () => boolean;
}) {
  const t = useT();
  const { preferences } = useStore();
  const row = useRef<HTMLLIElement>(null);
  useFocusLeaving(row, left);
  const failed = upload.failure !== undefined;
  const fixable = !isRefusal(upload.failure) || upload.failure.refused === "source-title";
  const name = upload.name || upload.file.name;
  return (
    <li ref={row} className="space-y-3 px-3 py-3">
      {fixable ? (
        <SourceTitleForm assets={assets} upload={upload} explain={explain} canRetry={canRetry} />
      ) : (
        <>
          <span className="block truncate">{upload.file.name}</span>
          <p role="alert" className="text-destructive">
            {failureText(upload.failure, t, preferences.locale, sourceMaxBytes)}
          </p>
        </>
      )}
      <div className="flex items-center justify-between gap-3">
        {!failed && (
          <output className="text-muted-foreground">
            {upload.preparing ? t("source.uploadPreparing") : t("source.uploadCreating")}
          </output>
        )}
        <Button
          variant="ghost"
          aria-label={failed ? t("asset.dismissUpload", { name }) : t("asset.cancelUpload", { name })}
          onClick={() => (failed ? assets.dismiss(upload) : upload.cancel())}
        >
          {failed ? t("asset.dismiss") : t("page.cancel")}
        </Button>
      </div>
    </li>
  );
});

const SourceTitleForm = observer(function SourceTitleForm({
  assets,
  upload,
  explain,
  canRetry,
}: {
  assets: AssetStore;
  upload: Upload;
  explain: (error: unknown) => ReactNode | undefined;
  canRetry: () => boolean;
}) {
  const t = useT();
  const { ref, sending, banner, problemOf, submit } = useForm(["title"], { ...sourceFormTexts, explain });
  const failed = upload.failure !== undefined;
  const titleRefusal =
    isRefusal(upload.failure) && upload.failure.refused === "source-title" ? upload.failure : undefined;
  const server = formErrors(titleRefusal === undefined ? upload.failure : undefined, t, ["title"], sourceFormTexts);
  const problem =
    problemOf("title") ?? (titleRefusal === undefined ? server.fields.title : t(titleMessages[titleRefusal.issue]));
  const note = explain(upload.failure) ?? banner ?? server.banner;

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!failed || sending || !canRetry()) {
      return;
    }
    const issue = sourceTitleProblem(upload.name);
    void submit(issue === undefined ? {} : { title: titleMessages[issue] }, () =>
      assets.retrySource(upload, upload.name, () => {
        if (!canRetry()) throw new ApiError(403, { code: "forbidden" });
      })
    );
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-3">
      {note !== undefined && <Alert>{note}</Alert>}
      <FormField
        label={t("source.uploadTitleFor", { name: upload.file.name })}
        name="title"
        value={upload.name}
        disabled={!failed || sending || !canRetry()}
        error={problem}
        onChange={(event) => runInAction(() => (upload.name = event.target.value))}
      />
      {failed && (
        <Button type="submit" disabled={sending || !canRetry()}>
          {t("source.uploadRetry")}
        </Button>
      )}
    </form>
  );
});

/**
 * failureText says why an upload failed: not sent, as the page knew; too large, by the instance's largest; its
 * names taken, the last it tried as well; or the error's text.
 */
function failureText(failure: unknown, t: Translate, locale: Locale, max: number | undefined): string {
  if (isRefusal(failure)) {
    switch (failure.refused) {
      case "page-file":
        return t("asset.pageFile");
      case "too-large":
        return t("asset.tooLarge", { max: formatBytes(failure.max, locale) });
      case "invalid-utf8":
        return t("source.uploadInvalidUtf8");
      case "source-title":
        return t(titleMessages[failure.issue]);
    }
  }
  if (failure instanceof ApiError && failure.code === "payload_too_large" && max !== undefined) {
    return t("asset.tooLarge", { max: formatBytes(max, locale) });
  }
  if (failure instanceof ApiError && failure.code === "page.title_taken") {
    return t("asset.nameTaken");
  }
  return errorText(failure, t) ?? t("asset.failed");
}
