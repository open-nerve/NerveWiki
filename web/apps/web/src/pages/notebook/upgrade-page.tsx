import { observer } from "mobx-react-lite";
import { Link } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { NotLoaded } from "../../app/not-loaded";
import { formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { Upgrade } from "../../services/upgrade.service";
import type { Workspace } from "../../services/workspace.service";
import { usePageTree, useStore, useUpgrades } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { distinctName } from "./distinct-name";
import { useNotebook } from "./notebook-layout";

/**
 * UpgradePage is the notebook's upgrade to an LLM Wiki under way (M11/P1
 * design 4.4), its admins' alone: to the others it says who can see it
 * and reads nothing (v0.1 design 13.2, item 18). Its title takes the focus
 * when the page is arrived at, from the start of the upgrade.
 */
export const UpgradePage = observer(function UpgradePage() {
  const notebook = useNotebook();
  const workspace = useWorkspace();
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  useDocumentTitle(t("upgrade.title"), notebook.name);
  return (
    <div className="max-w-2xl space-y-8">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
        {t("upgrade.title")}
      </h1>
      {notebook.role === "admin" ? (
        <UpgradeRead notebook={notebook} workspace={workspace} />
      ) : (
        <p>{t("upgrade.adminsOnly")}</p>
      )}
    </div>
  );
});

/**
 * UpgradeRead reads the upgrade (SWR's key ["upgrade", notebook id], which
 * the mode event and each connection read again): refused as forbidden,
 * the account is no longer the notebook's admin, and the workspace's
 * notebooks are read again, whose roles the page follows; none under way
 * says so.
 */
const UpgradeRead = observer(function UpgradeRead({
  notebook,
  workspace,
}: {
  notebook: Notebook;
  workspace: Workspace;
}) {
  const upgrades = useUpgrades();
  const { mutate } = useSWRConfig();
  const t = useT();
  const {
    data,
    error,
    mutate: reread,
  } = useSWR(["upgrade", notebook.id], () => upgrades.get(notebook.id), {
    onError: (refusal: unknown) => {
      if (refusal instanceof ApiError && refusal.code === "forbidden") {
        void mutate(["notebooks", workspace.id]);
      }
    },
  });
  if (error instanceof ApiError && error.code === "wiki.upgrade_not_found") {
    return <p>{t("problem.wiki.upgrade_not_found")}</p>;
  }
  if (data === undefined || error !== undefined) {
    return <NotLoaded error={error} retry={() => void reread()} />;
  }
  return <UpgradeState notebook={notebook} upgrade={data} />;
});

/**
 * UpgradeState is who started the upgrade and when, the source root it
 * proposes and the pages its start wrote, each leading to its page while
 * the tree has it.
 */
const UpgradeState = observer(function UpgradeState({ notebook, upgrade }: { notebook: Notebook; upgrade: Upgrade }) {
  const t = useT();
  const { slug } = useWorkspace();
  const { preferences } = useStore();
  const pages = usePageTree(notebook);
  useSWR(["pages", notebook.id], () => pages.load());
  const pageLink = (id: string) => {
    const name = distinctName(pages.tree, notebook, id, t);
    return name === undefined ? undefined : (
      <Link key={id} to={`/${slug}/notebooks/${notebook.id}/pages/${id}`} className="underline underline-offset-4">
        {name}
      </Link>
    );
  };
  const root = pageLink(upgrade.source_root_id);
  const seeded = upgrade.seeded_page_ids.flatMap((id) => pageLink(id) ?? []);
  return (
    <section className="space-y-2">
      <h2 className="text-lg font-semibold">{t("upgrade.state")}</h2>
      <p>
        {t("upgrade.startedBy", {
          name: upgrade.started_by.display_name,
          time: formatDateTime(upgrade.started_at, preferences.locale),
        })}
      </p>
      {root !== undefined && (
        <p>
          {t("upgrade.sourceRoot")} {root}
        </p>
      )}
      {seeded.length > 0 && (
        <p>
          {t("upgrade.seeded")}{" "}
          {seeded.map((link, at) => (
            <span key={link.key}>
              {at > 0 && ", "}
              {link}
            </span>
          ))}
        </p>
      )}
    </section>
  );
});
