import { observer } from "mobx-react-lite";
import { useEffect, useLayoutEffect, useRef } from "react";

import { errorText } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { formatBytes } from "../../i18n/format";
import { useT, type Translate } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import { ApiError } from "../../services/api";
import { isRefusal, type AssetStore, type Upload } from "../../stores/asset.store";
import { useStore } from "../../stores/context";

/**
 * UploadRows are the uploads under a page or the root (M7/P4 design 3.5),
 * above its attachments: each going with its progress, a bar and its
 * percentage, and Cancel; each sent whole, or answered, finishing, no
 * longer to be cancelled; each failed with why, and Dismiss. One button
 * does each in turn, so that the focus on it stays as the upload goes on;
 * a row that leaves with the focus in it gives it to left.
 */
export const UploadRows = observer(function UploadRows({
  assets,
  uploads,
  left,
}: {
  assets: AssetStore;
  uploads: readonly Upload[];
  left: () => void;
}) {
  const t = useT();
  return (
    <ul aria-label={t("asset.uploads")} className="divide-y rounded-md border text-sm">
      {uploads.map((upload) => (
        <UploadRow key={upload.key} assets={assets} upload={upload} left={left} />
      ))}
    </ul>
  );
});

const UploadRow = observer(function UploadRow({
  assets,
  upload,
  left,
}: {
  assets: AssetStore;
  upload: Upload;
  left: () => void;
}) {
  const t = useT();
  const { instance, preferences } = useStore();
  const row = useRef<HTMLLIElement>(null);
  const leaving = useRef(left);
  useEffect(() => {
    leaving.current = left;
  });
  // Before the row leaves the document: the focus in it goes elsewhere, not to the page's start.
  useLayoutEffect(() => {
    const element = row.current;
    return () => {
      if (element?.contains(document.activeElement)) {
        leaving.current();
      }
    };
  }, []);
  const failed = upload.failure !== undefined;
  const finishing = !failed && (upload.answered || (upload.total > 0 && upload.sent >= upload.total));
  const percent = upload.total > 0 ? Math.floor((upload.sent / upload.total) * 100) : 0;
  return (
    <li ref={row} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2">
      <span className="min-w-0 flex-1 truncate">{upload.name}</span>
      <span className="flex items-center gap-3">
        {failed ? (
          <span role="alert" className="text-destructive">
            {failureText(upload.failure, t, preferences.locale, instance.info?.asset_max_bytes)}
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
              ? undefined
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

/**
 * failureText says why an upload failed: not sent, as the page knew; too large, by the instance's largest; its
 * names taken, the last it tried as well; or the error's text.
 */
function failureText(failure: unknown, t: Translate, locale: Locale, max: number | undefined): string {
  if (isRefusal(failure)) {
    return failure.refused === "page-file"
      ? t("asset.pageFile")
      : t("asset.tooLarge", { max: formatBytes(failure.max, locale) });
  }
  if (failure instanceof ApiError && failure.code === "payload_too_large" && max !== undefined) {
    return t("asset.tooLarge", { max: formatBytes(max, locale) });
  }
  if (failure instanceof ApiError && failure.code === "page.title_taken") {
    return t("asset.nameTaken");
  }
  return errorText(failure, t) ?? t("asset.failed");
}
