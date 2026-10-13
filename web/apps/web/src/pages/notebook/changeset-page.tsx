import { observer } from "mobx-react-lite";
import { useId, useRef, useState, type RefObject } from "react";
import { Link, useParams } from "react-router";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";

import { useDocumentTitle } from "../../app/document-title";
import { writesPages } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { idPattern } from "../../lib/ids";
import { readPages } from "../../lib/read-pages";
import { shortId, twinsOf } from "../../lib/twins";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { ChangesetChangePage, ChangesetDetail } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { LoadMoreButton, useLoadMore } from "../load-more";
import { NotFoundPage } from "../not-found";
import { BreadcrumbTrail } from "../page/breadcrumbs";
import { useWorkspace } from "../workspace/workspace-layout";
import { ChangesetChangeRow } from "./changeset-change";
import { byText, changesetName, clientText, countsText } from "./changeset-text";
import { useNotebook } from "./notebook-layout";
import { RevertDialog, type Reverted } from "./revert-dialog";

/**
 * Said is what the changeset's page says of an undo of it: done, leading
 * to the undo; nothing to undo; its answer lost, finding out (unknown):
 * not found out (unread), undone by someone else (by), or not undone.
 */
type Said =
  | { kind: "reverted"; revert: string }
  | { kind: "unchanged" }
  | { kind: "unknown" }
  | { kind: "unread" }
  | { kind: "by"; name: string }
  | { kind: "notDone" };

/**
 * ChangesetPage is a changeset of a notebook's history (M8 design 4.7,
 * 4.13; M8/P3 design 4.3), by its id in the address: an address of no
 * changeset's id is no page, and is not read; nor is a changeset of
 * another notebook, or one not found.
 */
export const ChangesetPage = observer(function ChangesetPage() {
  const notebook = useNotebook();
  const { changesetId = "" } = useParams();
  if (!idPattern.test(changesetId)) {
    return <NotFoundPage />;
  }
  return <ChangesetShell key={changesetId} notebook={notebook} id={changesetId} />;
});

/**
 * ChangesetShell is the changeset found: where it is, under the history;
 * its kind and who made it, from where, when it was made and last
 * written, why, what it undid and what undid it, each leading there, why
 * it can no longer be undone when the purge trimmed it, its counts; a
 * writer's Undo change (revert-dialog.tsx), and what came of it, in the
 * status line or, refused, in an alert, which describe the title; then
 * the nodes it changed, a page of them at a time, More reading the next,
 * read again as many pages as were read (readPages). An undo whose answer
 * was lost reads the changeset again: a newest undo of it other than the
 * one known as it was sent is the account's, or someone else's.
 */
const ChangesetShell = observer(function ChangesetShell({ notebook, id }: { notebook: Notebook; id: string }) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { slug } = useWorkspace();
  const { locale } = useStore().preferences;
  const { data, error, mutate } = useSWR(["changeset", notebook.id, id], () => pages.changeset(id));
  const heading = useRef<HTMLHeadingElement>(null);
  const me = useStore().account?.me?.id;
  const [said, setSaid] = useState<Said>();
  const [refused, setRefused] = useState<unknown>();
  // The status line describes the title, where the focus lands: it is heard with it; a refusal's alert, likewise.
  const status = useId();
  const note = useId();
  const history = t("history.title");
  useDocumentTitle(data === undefined ? undefined : byText(data, t), history, notebook.name);
  // A changeset of another notebook is no page of this one's, nor is one not found.
  if ((data !== undefined && data.notebook_id !== notebook.id) || (error instanceof ApiError && error.status === 404)) {
    return <NotFoundPage />;
  }
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const home = `/${slug}/notebooks/${notebook.id}`;
  const href = (changeset: string) => `${home}/history/${changeset}`;
  const writer = writesPages(notebook.role);

  async function findOut(before: string | undefined): Promise<void> {
    setSaid({ kind: "unknown" });
    let found: Said;
    try {
      // Read here, not through SWR: its mutate answers what it holds when the read fails.
      const now = await pages.changeset(id);
      // It rejects only once the login's cache is gone, the page with it (M8/P1 fix check 3, R3-1).
      void mutate(now, { revalidate: false }).catch(() => undefined);
      const newest = now.reverted_by[0];
      if (newest === undefined || newest === before) {
        found = { kind: "notDone" };
      } else {
        const revert = await pages.changeset(newest);
        found =
          revert.created_by.user_id === me
            ? { kind: "reverted", revert: newest }
            : { kind: "by", name: revert.created_by.display_name };
      }
    } catch {
      found = { kind: "unread" };
    }
    setSaid(found);
  }

  function done(reverted: Reverted): void {
    switch (reverted.kind) {
      case "reverted":
        setSaid({ kind: "reverted", revert: reverted.revert });
        return;
      case "unchanged":
        setSaid({ kind: "unchanged" });
        return;
      case "refused":
        setRefused(reverted.error);
        return;
      case "unknown":
        void findOut(reverted.before);
    }
  }
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <BreadcrumbTrail
          steps={[
            { key: "notebook", to: home, name: notebook.name },
            { key: "history", to: `${home}/history`, name: history },
          ]}
          current={changesetName(data, false, t, locale)}
        />
        <h1
          ref={heading}
          tabIndex={-1}
          aria-describedby={refused !== undefined ? note : said !== undefined ? status : undefined}
          className="text-3xl font-semibold break-words outline-none"
        >
          {byText(data, t)}
        </h1>
        <ChangesetFacts changeset={data} href={href} />
      </div>
      {!data.trimmed &&
        (writer ? (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
            <RevertDialog
              notebook={notebook}
              changeset={data}
              heading={heading}
              sent={() => {
                setSaid(undefined);
                setRefused(undefined);
              }}
              done={done}
            />
            {data.reverted_by.length > 0 && <p className="text-sm text-muted-foreground">{t("history.revertAgain")}</p>}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">{t("history.readersNote")}</p>
        ))}
      {/* There before it says anything: a status that comes with its region is not always read. */}
      <output id={status} className="block text-sm empty:sr-only">
        {said !== undefined && <SaidText said={said} href={href} />}
      </output>
      {refused !== undefined && <Alert id={note}>{errorText(refused, t)}</Alert>}
      <ChangesetChanges notebook={notebook} id={id} heading={heading} />
    </div>
  );
});

/** SaidText is what said says, an undo done leading to its page. */
function SaidText({ said, href }: { said: Said; href: (id: string) => string }) {
  const t = useT();
  switch (said.kind) {
    case "reverted":
      return (
        <>
          {t("history.revertDone")}{" "}
          <Link to={href(said.revert)} className="underline underline-offset-4">
            {t("history.revertSee")}
          </Link>
        </>
      );
    case "unchanged":
      return t("history.revertUnchanged");
    case "unknown":
      return t("history.revertUnknown");
    case "unread":
      return t("history.revertUnknownStill");
    case "by":
      return t("history.revertedByOther", { name: said.name });
    case "notDone":
      return t("history.revertNotDone");
  }
}

/**
 * ChangesetFacts is what the history says of a changeset, line by line:
 * where it came from, when it was made and last written, in full; its
 * message; what it undid, leading there, or that the purge took it; the
 * reverts of it, newest first, each leading to its page; why it can no
 * longer be undone, trimmed; and its counts.
 */
function ChangesetFacts({ changeset, href }: { changeset: ChangesetDetail; href: (id: string) => string }) {
  const t = useT();
  const { locale } = useStore().preferences;
  const { reverts } = changeset;
  return (
    <div className="space-y-1 text-sm">
      <p className="text-muted-foreground">{t("history.clientOf", { client: clientText(changeset.client, t) })}</p>
      <p className="text-muted-foreground">
        {t("history.made", { time: formatDateTime(changeset.created_at, locale) })}
        {changeset.updated_at !== changeset.created_at && (
          <>
            {" · "}
            {t("history.updated", { time: formatDateTime(changeset.updated_at, locale) })}
          </>
        )}
      </p>
      {changeset.message !== null && (
        <p className="break-words">
          {t("history.messageOf")} <bdi>{changeset.message}</bdi>
        </p>
      )}
      {changeset.kind === "revert" &&
        (reverts === null ? (
          <p>{t("history.revertsPurged")}</p>
        ) : (
          <p>
            <Link to={href(reverts.id)} className="break-words underline underline-offset-4">
              {t("history.reverts", { name: changesetName(reverts, false, t, locale) })}
            </Link>
          </p>
        ))}
      {changeset.reverted_by.length > 0 && (
        <p className="flex flex-wrap gap-x-3">
          <span>{t("history.revertedBy")}</span>
          {changeset.reverted_by.map((revert) => (
            <Link key={revert} to={href(revert)} className="underline underline-offset-4">
              {t("history.revertLink", { id: shortId(revert) })}
            </Link>
          ))}
        </p>
      )}
      {changeset.trimmed && <p>{t("history.trimmedNote")}</p>}
      <p className="text-muted-foreground">{countsText(changeset, t, locale)}</p>
    </div>
  );
}

/**
 * ChangesetChanges are the nodes a changeset changed, by node id, a page
 * of them at a time (changeset-change.tsx), More reading the next, the
 * focus on the first it adds as it goes; those of one name, by the end of
 * their ids too.
 */
const ChangesetChanges = observer(function ChangesetChanges({
  notebook,
  id,
  heading,
}: {
  notebook: Notebook;
  id: string;
  heading: RefObject<HTMLHeadingElement | null>;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const key = ["changeset-changes", notebook.id, id];
  const { cache } = useSWRConfig();
  const cached = () => cache.get(unstable_serialize(key));
  const { data, error, mutate } = useSWR(key, () =>
    readPages((cursor) => pages.changes(id, cursor), (cached()?.data as ChangesetChangePage[] | undefined)?.length ?? 1)
  );
  // The link of the node the focus falls to as More goes.
  const focused = useRef<HTMLAnchorElement>(null);
  const cursor = data?.at(-1)?.next_cursor ?? undefined;
  const more = useLoadMore(
    async () => {
      const next = await pages.changes(id, cursor);
      // Added after the page it was read after, which a read meanwhile may have read again.
      const read = await mutate(
        (list) => (list !== undefined && list.at(-1)?.next_cursor === cursor ? [...list, next] : list),
        { revalidate: false }
      );
      if (cached()?.isValidating === true) {
        // A read out as it was added answers what is older than the addition: SWR drops it. Another reads it all.
        void mutate();
      }
      const added = read?.at(-1) === next;
      return {
        added: added ? next.data.map((change) => change.node.id) : [],
        whole: added && next.next_cursor === null,
        last: read?.at(-1)?.data.at(-1)?.node.id,
      };
    },
    () => focused.current,
    heading
  );
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const changes = data.flatMap((read) => read.data);
  const twins = twinsOf(
    changes.map((change) => change.node),
    (node) => node.name
  );
  return (
    <section className="space-y-3">
      <ul aria-label={t("history.changes")} className="divide-y rounded-md border">
        {changes.map((change) => (
          <ChangesetChangeRow
            key={change.node.id}
            notebook={notebook}
            change={change}
            twin={twins.has(change.node.id)}
            left={() => heading.current?.focus()}
            linkRef={change.node.id === more.focusing ? focused : undefined}
          />
        ))}
      </ul>
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {cursor !== undefined && <LoadMoreButton more={more} label={t("history.moreChanges")} />}
    </section>
  );
});
