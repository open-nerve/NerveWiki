import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { arrived, useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { canEditPageBody } from "../../app/source-permissions";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../components/ui/tabs";
import { formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { PageVersionContent, TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { useNotebook } from "../notebook/notebook-layout";
import { useWorkspace } from "../workspace/workspace-layout";
import { Breadcrumbs } from "./breadcrumbs";
import { Comparison } from "./comparison";
import { EditLockNote } from "./edit-lock-note";
import { PageGone, type Restored } from "./page-layout";
import { Mark, useVersions, versionBy } from "./page-versions";
import { VersionRestore } from "./version-restore";
import { useLeftBehind, VersionView } from "./version-view";

/** A revision as an address writes it: a whole number from 1, no longer than a number holds exactly. */
const revisionPattern = /^[1-9]\d{0,14}$/;

/**
 * unchanging is SWR's options for a version's content: it changes only
 * while it is being edited, which the events read again
 * (events/handlers.ts). One cached, its own revision's put there as the
 * address is replaced, is not read again as it shows, nor as the window
 * has the focus back.
 */
const unchanging = { revalidateIfStale: false, revalidateOnFocus: false };

/** The tabs of a version: its reading view, and its comparisons with the version before it and with the page now. */
type Tab = "read" | "previous" | "current";

/**
 * VersionPage is a version of a page (M8 design 4.3, 4.13), the version
 * that holds the address's revision. It finds the page in the notebook's
 * tree, as the page's shell does (page-layout.tsx): nothing until the tree
 * is read, and no page of the app's for a page the tree does not have, or
 * a revision no version could hold. It has the versions of the pages
 * alive alone: a page in the trash shows the version it was deleted at in
 * the trash's preview (trash-preview.tsx).
 */
export const VersionPage = observer(function VersionPage() {
  const notebook = useNotebook();
  const pages = usePageTree(notebook);
  const { pageId = "", revision = "" } = useParams();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  if (pages.tree === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const page = pages.byId(pageId);
  if (page === undefined || !revisionPattern.test(revision)) {
    return <PageGone notebook={notebook} page={pageId} />;
  }
  return <VersionShell key={page.id} notebook={notebook} page={page} revision={Number(revision)} />;
});

/**
 * VersionShell is the version found: where its page is, the version's
 * revision, when it was written, by whom and what its changeset was, as
 * the right column's list has them (page-versions.tsx), leading to the
 * changeset's page (M8/P3 design 4.5), and who is editing
 * the page, as on the page. The version answered holds the address's
 * revision but writes another, the later save of an edit session: the
 * address is replaced with its own, and what was read under the revision
 * the address had leaves SWR's cache (useLeftRevision). Its tabs, a named
 * group, show its reading view, at its own revision, what it changed from
 * the version before it (none for the version that created the page), and
 * what restoring it would change of the page now; each is read as its tab
 * is chosen, by its own key, and all of them leave SWR's cache as the
 * version's page is left (useLeftBehind). A writer restores a version
 * other than the newest (version-restore.tsx).
 */
const VersionShell = observer(function VersionShell({
  notebook,
  page,
  revision,
}: {
  notebook: Notebook;
  page: TreeNode;
  revision: number;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { slug } = useWorkspace();
  const { preferences } = useStore();
  const navigate = useNavigate();
  const { mutate } = useSWRConfig();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const mounted = useMounted();
  useLeftBehind(notebook.id, page.id);
  useLeftRevision(notebook.id, page.id, revision);
  const [tab, setTab] = useState<Tab>("read");
  // What a restore that left the dialog says: nothing changed, or the page is gone.
  const [said, setSaid] = useState("");
  const home = `/${slug}/notebooks/${notebook.id}`;
  const href = (id?: string) => (id === undefined ? home : `${home}/pages/${id}`);
  const version = useSWR(
    ["page-version", notebook.id, page.id, revision],
    () => pages.version(page.id, revision),
    unchanging
  );
  const newest = useVersions(notebook, page.id).data?.[0]?.data[0];
  const data = version.data;
  const at = data?.revision;
  // The version's own revision, as soon as it is known: the address's may be one its edit's session moved past.
  const title = t("version.title", { revision: at ?? revision });
  useDocumentTitle(title, page.name, notebook.name);
  const versionsOf = `${home}/pages/${page.id}/versions`;
  useEffect(() => {
    if (data !== undefined && at !== undefined && at !== revision) {
      // Read under its own revision already: the address replaced reads nothing again.
      void mutate(["page-version", notebook.id, page.id, at], data, { revalidate: false });
      void navigate(`${versionsOf}/${at.toString()}`, { replace: true });
    }
  }, [data, at, revision, mutate, navigate, notebook.id, page.id, versionsOf]);
  const shown = tab === "previous" && data?.base_revision === null ? "read" : tab;
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Breadcrumbs notebook={notebook} ancestors={pages.ancestorsOf(page.id)} page={page} href={href} part={title} />
        <div className="flex items-start justify-between gap-3">
          <h1 ref={heading} tabIndex={-1} className="text-3xl font-semibold break-words outline-none">
            {title}
          </h1>
          {canEditPageBody(pages, page.id) && data !== undefined && newest !== undefined && newest.id !== data.id && (
            <VersionRestore
              notebook={notebook}
              page={page}
              version={data}
              said={setSaid}
              restored={() => {
                // Gone to the page, arrived at, unless the user left meanwhile.
                if (mounted()) {
                  void navigate(href(page.id), { state: { ...arrived, restored: data.revision } satisfies Restored });
                }
              }}
            />
          )}
        </div>
        {data !== undefined && (
          <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
            <time dateTime={data.updated_at}>{formatDateTime(data.updated_at, preferences.locale)}</time>
            <span>
              ·{" "}
              <Link to={`${home}/history/${data.changeset.id}`} className="underline underline-offset-4">
                {versionBy(data, t)}
              </Link>
            </span>
            {newest?.id === data.id && <Mark>{t("version.current")}</Mark>}
            {data.editing && <Mark>{t("version.editing")}</Mark>}
          </p>
        )}
        <output className="block text-sm text-muted-foreground empty:sr-only">{said}</output>
      </div>
      <EditLockNote notebook={notebook} page={page} released={() => heading.current?.focus()} />
      {data === undefined ? (
        <NotLoaded error={version.error} retry={() => void version.mutate()} />
      ) : (
        <Tabs value={shown} onValueChange={(value) => setTab(value as Tab)} activationMode="manual">
          <TabsList aria-label={t("version.views")}>
            <TabsTrigger value="read">{t("version.read")}</TabsTrigger>
            {data.base_revision !== null && <TabsTrigger value="previous">{t("version.comparePrevious")}</TabsTrigger>}
            <TabsTrigger value="current">{t("version.compareCurrent")}</TabsTrigger>
          </TabsList>
          <TabsContent value="read" className="pt-4">
            <VersionView
              notebook={notebook}
              page={page.id}
              revision={data.revision}
              label={t("version.title", { revision: data.revision })}
            />
          </TabsContent>
          {data.base_revision !== null && (
            <TabsContent value="previous" className="pt-4">
              <PreviousDiff notebook={notebook} page={page} version={data} base={data.base_revision} />
            </TabsContent>
          )}
          <TabsContent value="current" className="pt-4">
            <CurrentDiff notebook={notebook} page={page} version={data} />
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
});

/** PreviousDiff is what the version changed from the one before it, which holds its base revision. */
function PreviousDiff({
  notebook,
  page,
  version,
  base,
}: {
  notebook: Notebook;
  page: TreeNode;
  version: PageVersionContent;
  base: number;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { data, error, mutate } = useSWR(
    ["page-version", notebook.id, page.id, base],
    () => pages.version(page.id, base),
    unchanging
  );
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  return (
    <Comparison
      before={data.content}
      after={version.content}
      label={t("version.previousDiff", { base: data.revision, revision: version.revision })}
      beforeName={t("version.title", { revision: data.revision })}
      afterName={t("version.title", { revision: version.revision })}
    />
  );
}

/** CurrentDiff is what restoring the version would change of the page now: from its content now to the version's. */
function CurrentDiff({ notebook, page, version }: { notebook: Notebook; page: TreeNode; version: PageVersionContent }) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { data, error, mutate } = useSWR(["page-content", notebook.id, page.id], () => pages.content(page.id));
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  return (
    <Comparison
      before={data.content}
      after={version.content}
      label={t("version.currentDiff", { revision: version.revision })}
      beforeName={t("version.now")}
      afterName={t("version.title", { revision: version.revision })}
    />
  );
}

/**
 * useLeftRevision takes the content and the view read under the address's
 * revision out of SWR's cache once the address has another (M8/P1 review
 * B, I1): replaced by the revision of the version that holds it, an edit's
 * save after an edit's save, or another version's, they would stay there,
 * each up to 5 MiB, as long as the version's page is open. A key shown
 * still, the base revision the newer version is compared with, is read
 * again.
 */
function useLeftRevision(notebook: string, page: string, revision: number): void {
  const { mutate } = useSWRConfig();
  const shown = useRef(revision);
  useEffect(() => {
    const left = shown.current;
    shown.current = revision;
    if (left !== revision) {
      void mutate(
        (key) =>
          Array.isArray(key) &&
          (key[0] === "page-version" || key[0] === "page-version-view") &&
          key[1] === notebook &&
          key[2] === page &&
          key[3] === left,
        undefined
      );
    }
  }, [mutate, notebook, page, revision]);
}
