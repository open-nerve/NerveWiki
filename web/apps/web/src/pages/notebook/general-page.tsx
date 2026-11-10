import { observer } from "mobx-react-lite";
import { useRef, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { arrived } from "../../app/arrival";
import { ConfirmDialog } from "../../app/confirm-dialog";
import { useForm } from "../../app/form";
import { notebookNameProblem, notebookNameTexts } from "../../app/notebook-name";
import { useMounted } from "../../app/mounted";
import { pagesLocked } from "../../app/pages-locked";
import { RenameForm } from "../../app/rename-form";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { titleKey } from "../../lib/title-key";
import type { Notebook, WorkspaceAccess } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import type { Workspace } from "../../services/workspace.service";
import { useAccount, useNotebooks, usePageTree, useStore } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { AccessOptions } from "./access-options";
import { distinctName } from "./distinct-name";
import { useNotebook } from "./notebook-layout";
import { upgradeStateText } from "./upgrade-banner";

/**
 * NotebookGeneralPage is a notebook's name, workspace access and mode
 * (M3/P4 design 3.4, M10/P1 design 4.4): its admins change them and delete
 * it; the others see them.
 */
export const NotebookGeneralPage = observer(function NotebookGeneralPage() {
  const workspace = useWorkspace();
  const notebook = useNotebook();
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const admin = notebook.role === "admin";
  return (
    <div className="space-y-10">
      <section className="max-w-md space-y-6">
        <h2 className="text-lg font-semibold">{t("notebookSettings.general")}</h2>
        {admin ? (
          <>
            <RenameForm
              current={notebook.name}
              label={t("notebooks.name")}
              hint={t("notebooks.nameHint")}
              autoComplete="off"
              check={notebookNameProblem}
              fieldTexts={notebookNameTexts}
              rename={(name) => notebooks.update(notebook.id, { name })}
              saveLabel={t("notebookSettings.save")}
              savedLabel={t("notebookSettings.saved")}
            />
            <AccessForm workspace={workspace} notebook={notebook} />
          </>
        ) : (
          <>
            <div className="space-y-1">
              <h3 className="text-sm font-medium">{t("notebooks.name")}</h3>
              <p className="break-words">{notebook.name}</p>
              <p className="text-sm text-muted-foreground">{t("notebookSettings.nameAdminsOnly")}</p>
            </div>
            <div className="space-y-1">
              <h3 className="text-sm font-medium">{t("access.legend")}</h3>
              <p>{t(`access.${notebook.workspace_access}`)}</p>
              <p className="text-sm text-muted-foreground">{t(`access.${notebook.workspace_access}Hint`)}</p>
              <p className="text-sm text-muted-foreground">{t("notebookSettings.accessAdminsOnly")}</p>
            </div>
          </>
        )}
      </section>
      <ModeSection workspace={workspace} notebook={notebook} />
      {admin && <DeleteSection workspace={workspace} notebook={notebook} />}
    </div>
  );
});

type NotebookProps = { workspace: Workspace; notebook: Notebook };

/**
 * AccessForm changes who in the workspace sees the notebook: the access
 * chosen goes out on Save, not as it is chosen, since the arrow keys choose
 * as they move. Only a change goes out; the left column may then show the
 * notebook in the other group. Like RenameForm, it shows the access as the
 * list has it until one is chosen, and again once saved, unless another
 * was chosen while the save was out.
 */
const AccessForm = observer(function AccessForm({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  /** The access chosen since the last save; none, the access as the list has it. */
  const [draft, setDraft] = useState<WorkspaceAccess>();
  const access = draft ?? notebook.workspace_access;
  const [saved, setSaved] = useState(false);
  const { ref, sending, banner, submit } = useForm<never>([]);
  /** How many times the access was chosen: a save tells whether the access it sent is still the one shown. */
  const edits = useRef(0);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const chosen = access;
    const edit = edits.current;
    setSaved(false);
    void submit({}, async () => {
      if (chosen !== notebook.workspace_access) {
        await notebooks.update(notebook.id, { workspace_access: chosen });
      }
      if (edits.current === edit) {
        setDraft(undefined);
        setSaved(true);
      }
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <AccessOptions
        value={access}
        onChange={(chosen) => {
          edits.current++;
          setDraft(chosen);
          setSaved(false);
        }}
      />
      <div className="flex items-center gap-3">
        <Button type="submit" disabled={sending}>
          {t("notebookSettings.save")}
        </Button>
        <output className="text-sm text-muted-foreground">{saved ? t("notebookSettings.saved") : ""}</output>
      </div>
    </form>
  );
});

/**
 * ModeSection is the notebook's mode (M10/P1 design 4.4) and its upgrade
 * (M11/P1 design 4.4). An admin downgrades an LLM Wiki; an admin starts
 * upgrading a standard notebook without an upgrade, once its tree is
 * read: what the start makes of a page raw at the root comes from it.
 * Each dialog stays, with its refusal, when a read again removes its
 * trigger. Started, the upgrade's page shows, its title taking the
 * focus, with the upgrade the start answered until it is read. While an
 * upgrade is under way the section says where it is, and its admins get
 * its page (decision 246).
 */
const ModeSection = observer(function ModeSection({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const pages = usePageTree(notebook);
  const { me } = useAccount();
  const { preferences } = useStore();
  const { mutate } = useSWRConfig();
  const navigate = useNavigate();
  const mounted = useMounted();
  const t = useT();
  const [open, setOpen] = useState(false);
  const [starting, setStarting] = useState(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const startButton = useRef<HTMLButtonElement>(null);
  // The left column reads the same tree: what the start makes of raw is counted in it.
  useSWR(["pages", notebook.id], () => pages.load());
  const wiki = notebook.mode === "llm_wiki";
  const admin = notebook.role === "admin";
  const downgrades = wiki && admin;
  const upgrade = notebook.upgrade;
  const upgrades = !wiki && admin && upgrade === null && pages.tree !== undefined;
  const raw = pages.childrenOf(null).find((node) => titleKey(node.name) === titleKey("raw"));
  const underRaw = raw === undefined ? 0 : nodesUnder(pages.nodes ?? [], raw.id);
  const home = `/${workspace.slug}/notebooks/${notebook.id}`;

  function onClosed(done: boolean) {
    (done ? heading.current : (button.current ?? heading.current))?.focus();
  }

  return (
    <section className="max-w-md space-y-3">
      <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">
        {t("notebookSettings.mode")}
      </h2>
      <p>{wiki ? t("notebookSettings.modeLLMWiki") : t("notebookSettings.modeStandard")}</p>
      <p className="text-sm text-muted-foreground">
        {wiki ? t("notebookSettings.modeLLMWikiHint") : t("notebookSettings.modeStandardHint")}
      </p>
      {upgrade !== null && (
        <p className="text-sm">
          {t(upgradeStateText[upgrade.state])}
          {admin && (
            <>
              {" "}
              <Link to={`${home}/upgrade`} className="font-medium underline underline-offset-4">
                {t("upgrade.view")}
              </Link>
            </>
          )}
        </p>
      )}
      {downgrades && (
        <Button ref={button} variant="outline" onClick={() => setOpen(true)}>
          {t("notebookSettings.downgrade")}
        </Button>
      )}
      {upgrades && (
        <Button ref={startButton} variant="outline" onClick={() => setStarting(true)}>
          {t("upgrade.start")}
        </Button>
      )}
      <ConfirmDialog
        held={{ open, onOpenChange: setOpen, onClosed }}
        title={t("notebookSettings.downgradeTitle", { name: notebook.name })}
        description={t("notebookSettings.downgradeBody")}
        confirmLabel={t("notebookSettings.downgradeConfirm")}
        sendingLabel={t("notebookSettings.downgrading")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={() => notebooks.downgrade(notebook.id)}
        texts={{ "transfer.busy": "transfer.writingBusy", forbidden: "notebookSettings.downgradeAdminsOnly" }}
        tone="default"
      />
      <ConfirmDialog
        held={{
          open: starting,
          onOpenChange: setStarting,
          onClosed: (done) => (done ? heading.current : (startButton.current ?? heading.current))?.focus(),
        }}
        title={t("upgrade.startTitle", { name: notebook.name })}
        description={t("upgrade.startBody")}
        more={
          underRaw > 0 ? (
            <p>{t("upgrade.startRaw", { count: new Intl.NumberFormat(preferences.locale).format(underRaw) })}</p>
          ) : undefined
        }
        confirmLabel={t("upgrade.startConfirm")}
        sendingLabel={t("upgrade.starting")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={async () => {
          const started = await notebooks.startUpgrade(notebook.id, preferences.locale);
          // The upgrade's page shows what the start answered, not what was read before it; it reads it as it opens.
          void mutate(["upgrade", notebook.id], started, { revalidate: false });
          // The settings left meanwhile stay left: the upgrade shows in the banner and the list.
          if (mounted()) {
            void navigate(`${home}/upgrade`, { state: arrived });
          }
        }}
        explain={(error) => pagesLocked(error, t, me.id, (id) => distinctName(pages.tree, notebook, id, t))}
        texts={{ "transfer.busy": "transfer.writingBusy" }}
        tone="default"
      />
    </section>
  );
});

/**
 * nodesUnder counts the nodes, pages and attachments, under the node id
 * among nodes, at any depth: linear in nodes, none spread into arguments
 * (v0.1 design 13.2, item 25).
 */
function nodesUnder(nodes: readonly TreeNode[], id: string): number {
  const children = new Map<string | null, string[]>();
  for (const node of nodes) {
    const siblings = children.get(node.parent_id);
    if (siblings === undefined) {
      children.set(node.parent_id, [node.id]);
    } else {
      siblings.push(node.id);
    }
  }
  let count = 0;
  const below = [id];
  for (let at = below.pop(); at !== undefined; at = below.pop()) {
    for (const child of children.get(at) ?? []) {
      count += 1;
      below.push(child);
    }
  }
  return count;
}

/**
 * DeleteSection deletes the notebook for every member, once its name is
 * typed (M3 design 4); the shell then goes to the workspace's home,
 * arrived at.
 */
function DeleteSection({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  return (
    <section className="max-w-md space-y-3">
      <h2 className="text-lg font-semibold">{t("notebookSettings.deleteTitle")}</h2>
      <p className="text-sm text-muted-foreground">{t("notebookSettings.deleteBody")}</p>
      <ConfirmDialog
        trigger={<Button variant="destructive">{t("notebookSettings.delete")}</Button>}
        title={t("notebookSettings.deleteConfirmTitle", { name: notebook.name })}
        description={t("notebookSettings.deleteBody")}
        typedConfirmation={{ label: t("notebookSettings.typeName", { name: notebook.name }), value: notebook.name }}
        confirmLabel={t("notebookSettings.deleteConfirm")}
        sendingLabel={t("notebookSettings.deleting")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={() => notebooks.remove(notebook.id)}
      />
    </section>
  );
}
