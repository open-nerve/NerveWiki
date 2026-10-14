import { observer } from "mobx-react-lite";
import { useRef, type Ref } from "react";
import { Link } from "react-router";

import { Button } from "../../components/ui/button";
import { formatDateTime, formatRelative } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { shortId } from "../../lib/twins";
import type { Changeset } from "../../services/page.service";
import { useStore } from "../../stores/context";
import { Mark } from "../page/page-versions";
import { byText, changesetName, changesText, clientText, countsText } from "./changeset-text";

/**
 * HistoryRow is a changeset of a notebook's history (M8 design 4.7, 4.13;
 * M8/P3 design 4.2): its kind and who made it, which leads to its page and
 * is named by when it was made, to the minute, a twin by the end of its id
 * too, which the row shows (twin); Only this person beside them, unless
 * the list is theirs already (only), named by who, an account of the name
 * of another told apart as the filter tells it; where it came from, as long ago
 * before now as it is (the time in full as its title), why (its message),
 * its first changes summed up and its counts; and its marks: undone,
 * leading to its newest revert, trimmed, and what a revert undid, leading
 * there. Gone with the focus, read away, it gives the focus away (left)
 * before it leaves.
 */
export const HistoryRow = observer(function HistoryRow({
  changeset,
  href,
  twin,
  revertsTwin,
  only,
  left,
  linkRef,
  now,
}: {
  changeset: Changeset;
  /** The address of the page of the changeset id. */
  href: (id: string) => string;
  /** Whether another changeset the page names has its name, and that of the one it reverts. */
  twin: boolean;
  revertsTwin: boolean;
  /** Lists the changesets of the account who made it alone, its name; none when the list is theirs already. */
  only: { name: string; choose: () => void } | undefined;
  left: () => void;
  linkRef?: Ref<HTMLAnchorElement>;
  /** The time the list was shown at, which its changesets' times are said before. */
  now: number;
}) {
  const t = useT();
  const { locale } = useStore().preferences;
  const row = useRef<HTMLLIElement>(null);
  useFocusLeaving(row, left);
  const name = changesetName(changeset, twin, t, locale);
  const newest = changeset.reverted_by[0];
  const reverts = changeset.reverts;
  const summary = changesText(changeset, t, locale);
  return (
    <li ref={row} className="space-y-1 p-4">
      <p className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <Link
          ref={linkRef}
          to={href(changeset.id)}
          aria-label={name}
          className="font-medium break-words underline underline-offset-4"
        >
          {byText(changeset, t)}
        </Link>
        {only !== undefined && (
          <Button variant="ghost" className="h-7 px-2 text-xs" aria-label={only.name} onClick={only.choose}>
            {t("history.only")}
          </Button>
        )}
      </p>
      <p className="text-sm break-words text-muted-foreground">
        {clientText(changeset.client, t)}
        {" · "}
        <time dateTime={changeset.created_at} title={formatDateTime(changeset.created_at, locale)}>
          {formatRelative(changeset.created_at, now, locale)}
        </time>
        {twin && (
          <>
            {" · "}
            {t("history.id", { id: shortId(changeset.id) })}
          </>
        )}
      </p>
      {changeset.message !== null && (
        <p className="text-sm break-words">
          <bdi>{changeset.message}</bdi>
        </p>
      )}
      {summary !== "" && <p className="text-sm break-words">{summary}</p>}
      <p className="text-sm text-muted-foreground">{countsText(changeset, t, locale)}</p>
      {(newest !== undefined || changeset.trimmed || changeset.kind === "revert") && (
        <p className="flex flex-wrap items-center gap-2 text-sm">
          {newest !== undefined && (
            <Link
              to={href(newest)}
              aria-label={t("history.revertedLabel", { name })}
              className="rounded border px-1.5 text-xs underline underline-offset-4"
            >
              {t("history.reverted")}
            </Link>
          )}
          {changeset.trimmed && <Mark>{t("history.trimmed")}</Mark>}
          {changeset.kind === "revert" &&
            (reverts === null ? (
              <span className="text-muted-foreground">{t("history.revertsPurged")}</span>
            ) : (
              <Link to={href(reverts.id)} className="break-words underline underline-offset-4">
                {t("history.reverts", { name: changesetName(reverts, revertsTwin, t, locale) })}
              </Link>
            ))}
        </p>
      )}
    </li>
  );
});
