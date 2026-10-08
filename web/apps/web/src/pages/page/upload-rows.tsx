import { observer } from "mobx-react-lite";

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
 * percentage, and Cancel; each failed with why, and Dismiss. Either button
 * leaves the row, the focus going to left.
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
  const { instance, preferences } = useStore();
  return (
    <ul aria-label={t("asset.uploads")} className="divide-y rounded-md border text-sm">
      {uploads.map((upload) => {
        const percent = upload.total > 0 ? Math.floor((upload.sent / upload.total) * 100) : 0;
        return (
          <li key={upload.key} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2">
            <span className="min-w-0 flex-1 truncate">{upload.name}</span>
            {upload.failure === undefined ? (
              <>
                <progress
                  value={upload.sent}
                  max={Math.max(upload.total, 1)}
                  aria-label={t("asset.progressOf", { name: upload.name })}
                  className="h-2 w-24"
                />
                <span className="w-10 text-right text-xs text-muted-foreground tabular-nums">{`${percent}%`}</span>
                <Button
                  variant="ghost"
                  aria-label={t("asset.cancelUpload", { name: upload.name })}
                  onClick={() => {
                    upload.cancel();
                    left();
                  }}
                >
                  {t("page.cancel")}
                </Button>
              </>
            ) : (
              <>
                <span role="alert" className="basis-full text-destructive sm:basis-auto">
                  {failureText(upload.failure, t, preferences.locale, instance.info?.asset_max_bytes)}
                </span>
                <Button
                  variant="ghost"
                  aria-label={t("asset.dismissUpload", { name: upload.name })}
                  onClick={() => {
                    assets.dismiss(upload);
                    left();
                  }}
                >
                  {t("asset.dismiss")}
                </Button>
              </>
            )}
          </li>
        );
      })}
    </ul>
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
