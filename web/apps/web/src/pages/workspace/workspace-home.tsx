import { Book, Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { Link } from "react-router";
import useSWR from "swr";

import { arrived, useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { useFollowRole } from "../../app/follow-role";
import { useMounted } from "../../app/mounted";
import { groupTitleClass, homeColumnClass } from "../../app/page-classes";
import { badgeClass, pageTitleClass, WholeParts } from "../../app/settings-page";
import { EmptyState } from "../../components/empty-state";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks, useOwnerless, useStore } from "../../stores/context";
import { createsNotebooks } from "../../stores/notebook.store";
import { CreateNotebookDialog } from "./create-notebook-dialog";
import { NotebookGroups } from "./notebook-groups";
import { useWorkspace } from "./workspace-layout";

/** The cards' grid: one track written at its narrowest, an auto one widened by a name cut short (decision 917). */
const gridClass = "grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3";

/**
 * WorkspaceHomePage is a workspace's first page (M3/P4 design 3.3): its
 * notebooks in their two groups, as cards (M13/P5/S1, decisions 916, 917),
 * then a dashed New notebook in a row of its own for the workspace's
 * admins and members (decisions 918, 942): in no group, it is no notebook;
 * with none, the empty state offers it. Its admins read there how many
 * notebooks have no admin, if any (M3/P5 design 3.3). Centred, at most
 * 928px wide: three cards abreast from xl.
 */
export const WorkspaceHomePage = observer(function WorkspaceHomePage() {
  const workspace = useWorkspace();
  const notebooks = useNotebooks(workspace);
  const { preferences } = useStore();
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  useDocumentTitle(workspace.name);
  const here = useMounted();
  const members = (n: number) =>
    n === 1 ? t("notebooks.membersOne") : t("notebooks.members", { count: n.toLocaleString(preferences.locale) });
  // The list read and not empty is both groups not empty: groupNotebooks parts it (NotebookGroups reads it too).
  const listed = notebooks.list !== undefined && notebooks.list.length > 0;
  return (
    <section className={homeColumnClass}>
      <h1 ref={heading} tabIndex={-1} className={pageTitleClass}>
        {workspace.name}
      </h1>
      {workspace.role === "admin" && <OwnerlessReminder workspace={workspace} />}
      <NotebookGroups
        workspace={workspace}
        headingClassName={groupTitleClass}
        listClassName={gridClass}
        groupClassName="min-w-0"
        renderItem={(notebook) => (
          // The card leaves with the home: the notebook's home is arrived at, its title taking the focus (v0.1
          // design 13.2, item 17; M12/P2/S1). A link: the focus is the global outline (v0.1 design 13.2, item 31).
          <Link
            to={`/${workspace.slug}/notebooks/${notebook.id}`}
            state={arrived}
            className="flex h-full flex-col gap-3 rounded-card border border-border bg-surface p-4 transition-colors duration-150 hover:border-border-strong hover:hover-layer sm:min-h-28"
          >
            {/* The name as wide as its words, the badge right after it (decision 947); the tile bordered, transparent but
              in forced colours, where its fill is gone (P5S1B-N2). */}
            <span className="flex min-w-0 items-center gap-2.5">
              <span
                aria-hidden
                className="grid size-8 shrink-0 place-items-center rounded-control border border-transparent bg-surface-2 text-muted-foreground [&_svg]:size-4"
              >
                <Book />
              </span>
              <span title={notebook.name} className="min-w-0 truncate text-sm font-medium">
                {notebook.name}
              </span>
              {notebook.mode === "llm_wiki" && (
                <span className={cn(badgeClass, "ms-0 shrink-0")}>{t("notebooks.modeLLMWiki")}</span>
              )}
            </span>
            <span className="mt-auto text-[12.5px] text-faint">
              <WholeParts parts={[t(`access.${notebook.workspace_access}`), members(notebook.member_count)]} />
            </span>
          </Link>
        )}
        empty={
          <EmptyState
            icon={Book}
            action={
              createsNotebooks(workspace) ? (
                <CreateNotebookDialog
                  workspace={workspace}
                  here={here}
                  trigger={
                    // A centred action, a small pill as a status page's (decision 949).
                    <Button size="sm" shape="pill">
                      {t("notebooks.create")}
                    </Button>
                  }
                />
              ) : undefined
            }
          >
            {t("notebooks.empty")}
          </EmptyState>
        }
      />
      {createsNotebooks(workspace) && listed && (
        <div className={gridClass}>
          <CreateNotebookDialog
            workspace={workspace}
            here={here}
            trigger={
              // A control: its focus the ring, as a Button's (v0.1 design 13.2, item 31).
              <button
                type="button"
                className="flex min-h-14 w-full items-center justify-center gap-2 rounded-card border border-dashed border-border-strong text-sm font-medium text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden sm:min-h-28 [&_svg]:size-4"
              >
                <Plus aria-hidden />
                {t("notebooks.create")}
              </button>
            }
          />
        </div>
      )}
    </section>
  );
});

/**
 * OwnerlessReminder says how many of the workspace's notebooks have no
 * admin, with a way to them; nothing when none has, or while they are not
 * read: it is no part of the home's own content. It reads them as the
 * ownerless page does, once for both.
 */
const OwnerlessReminder = observer(function OwnerlessReminder({ workspace }: { workspace: Workspace }) {
  const ownerless = useOwnerless(workspace);
  const t = useT();
  useSWR(["ownerless", workspace.id], () => ownerless.load(), { onError: useFollowRole() });
  const count = ownerless.list?.length ?? 0;
  if (count === 0) {
    return null;
  }
  return (
    <p className="text-sm text-muted-foreground">
      {t("ownerless.reminder", { count })}{" "}
      {/* It leaves the workspace's shell for the settings': arrived at, their page's title takes the focus (M13/P5/S1,
          decision 924). */}
      <Link
        to={`/${workspace.slug}/settings/ownerless`}
        state={arrived}
        className="font-medium text-foreground underline underline-offset-4"
      >
        {t("ownerless.review")}
      </Link>
    </p>
  );
});
