import { observer } from "mobx-react-lite";
import { useId, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { arrived } from "../../app/arrival";
import { ConfirmDialog } from "../../app/confirm-dialog";
import { useForm } from "../../app/form";
import { NameRow } from "../../app/name-row";
import { notebookNameProblem, notebookNameTexts } from "../../app/notebook-name";
import { useMounted } from "../../app/mounted";
import { pagesLocked } from "../../app/pages-locked";
import {
  rowClass,
  rowControlClass,
  rowDescriptionClass,
  rowTextClass,
  rowTitleClass,
  SavedOutput,
  SettingsCard,
  SettingsRow,
  SettingsSection,
} from "../../app/settings-page";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { NativeSelect, useChoice } from "../../components/ui/native-select";
import { useT } from "../../i18n/i18n";
import { titleKey } from "../../lib/title-key";
import type { Notebook, WorkspaceAccess } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import type { Workspace } from "../../services/workspace.service";
import { useAccount, useNotebooks, usePageTree, useStore } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { accesses } from "./access-options";
import { distinctName } from "./distinct-name";
import { useNotebook } from "./notebook-layout";
import { upgradeStateText } from "./upgrade-banner";

/**
 * NotebookGeneralPage is a notebook's name, workspace access and mode
 * (M3/P4 design 3.4, M10/P1 design 4.4): its admins change them and delete
 * it; the others see them. The basics' card first (M13/P3/S2, decisions
 * 820, 844): the name saved as it is changed (NameRow), the access as it
 * is chosen (AccessRow); the others' rows show each as a value. Then the mode's; the deletion the
 * danger zone's, last (decision 826).
 */
export const NotebookGeneralPage = observer(function NotebookGeneralPage() {
  const workspace = useWorkspace();
  const notebook = useNotebook();
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const admin = notebook.role === "admin";
  return (
    <>
      <SettingsSection title={t("settings.basics")}>
        <SettingsCard>
          {admin ? (
            <>
              <NameRow
                current={notebook.name}
                field="name"
                label={t("notebooks.name")}
                description={t("notebooks.nameHint")}
                autoComplete="off"
                check={notebookNameProblem}
                fieldTexts={notebookNameTexts}
                save={(name) => notebooks.update(notebook.id, { name })}
                saveLabel={t("notebookSettings.save")}
                savedLabel={t("notebookSettings.saved")}
              />
              <AccessRow workspace={workspace} notebook={notebook} />
            </>
          ) : (
            <>
              <SettingsRow
                title={t("notebooks.name")}
                description={t("notebookSettings.nameAdminsOnly")}
                value={notebook.name}
              />
              <SettingsRow
                title={t("access.legend")}
                description={
                  <>
                    {t(`access.${notebook.workspace_access}Hint`)}
                    {/* A sentence of its own, which is read whole (review C's R1-N4). */}
                    <span className="block">{t("notebookSettings.accessAdminsOnly")}</span>
                  </>
                }
                value={t(`access.${notebook.workspace_access}`)}
              />
            </>
          )}
        </SettingsCard>
      </SettingsSection>
      <ModeSection workspace={workspace} notebook={notebook} />
      {admin && <DangerSection workspace={workspace} notebook={notebook} />}
    </>
  );
});

type NotebookProps = { workspace: Workspace; notebook: Notebook };

/**
 * AccessRow changes who in the workspace sees the notebook, a select whose
 * choice goes out as it is made (M13/P3/S2, decisions 825, 840): the
 * pointer's at once, a step of the keys once the select is left or Enter
 * pressed, Escape taking the step back (useChoice). The choice that went
 * out is shown, its description too, until the latest's answer: only the
 * latest counts, an answer before it neither clearing what is shown nor
 * saying it was refused; refused, the select shows the list's access again
 * and the reason shows under the row, a refusal as forbidden saying who may
 * (texts). The select is never disabled while a choice is out, which would
 * drop the focus: busy instead, until the latest's answer. The left column
 * may then show the notebook in the other group. The notebook's changes go
 * in turn (NotebookStore's inTurn): the latest goes last, its answer the
 * list's at the end.
 */
const AccessRow = observer(function AccessRow({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const id = useId();
  /** The access chosen, shown until the server answers the latest choice; none, the list's. */
  const [draft, setDraft] = useState<WorkspaceAccess>();
  const shown = draft ?? notebook.workspace_access;
  const [saved, setSaved] = useState(false);
  const { ref, banner, submit } = useForm<never>([], {
    texts: { forbidden: "notebookSettings.accessAdminsOnly" },
  });
  /** How many choices went out: only the latest's answer shows. */
  const sent = useRef(0);
  const choice = useChoice(shown, (value) => {
    const chosen = accesses.find((access) => access.value === value)?.value;
    if (chosen === undefined) {
      return;
    }
    sent.current += 1;
    const mine = sent.current;
    setDraft(chosen);
    setSaved(false);
    void submit({}, async () => {
      try {
        await notebooks.update(notebook.id, { workspace_access: chosen });
      } catch (error) {
        // A refusal of a choice made before the latest says nothing: the latest's answer is what counts.
        if (sent.current !== mine) {
          return;
        }
        setDraft(undefined);
        throw error;
      }
      if (sent.current === mine) {
        setDraft(undefined);
        setSaved(true);
      }
    });
  });
  // What the select shows: a step of the keys too, waiting (decision 840).
  const hint = accesses.find((access) => access.value === choice.value)?.hint;
  return (
    <form ref={ref} noValidate onSubmit={(event) => event.preventDefault()}>
      <div className={rowClass}>
        <div className={rowTextClass}>
          <label htmlFor={id} className={rowTitleClass}>
            {t("access.legend")}
          </label>
          <p id={`${id}-hint`} className={rowDescriptionClass}>
            {hint === undefined ? "" : t(hint)}
          </p>
        </div>
        <div className={rowControlClass}>
          <SavedOutput saved={saved} label={t("notebookSettings.saved")} />
          {/* Busy while the latest choice is out: the draft's, not the form's sending, which the first answer of two
              ends (review B's R1-N2). */}
          <NativeSelect
            id={id}
            aria-describedby={`${id}-hint`}
            aria-busy={draft !== undefined || undefined}
            // Below md the row's whole width, as the name's field is (M13 closeout, decision 970).
            className="h-7.5 max-md:w-full md:text-compact"
            {...choice}
          >
            {accesses.map((access) => (
              <option key={access.value} value={access.value}>
                {t(access.label)}
              </option>
            ))}
          </NativeSelect>
        </div>
      </div>
      {banner !== undefined && (
        <div className="px-4.5 pb-3.5">
          <Alert>{banner}</Alert>
        </div>
      )}
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
    <SettingsSection title={t("notebookSettings.mode")} heading={heading}>
      <SettingsCard>
        <div>
          <SettingsRow
            title={wiki ? t("notebookSettings.modeLLMWiki") : t("notebookSettings.modeStandard")}
            description={wiki ? t("notebookSettings.modeLLMWikiHint") : t("notebookSettings.modeStandardHint")}
          >
            {downgrades || upgrades
              ? () => (
                  <>
                    {downgrades && (
                      <Button ref={button} variant="outline" size="sm" shape="pill" onClick={() => setOpen(true)}>
                        {t("notebookSettings.downgrade")}
                      </Button>
                    )}
                    {upgrades && (
                      <Button
                        ref={startButton}
                        variant="outline"
                        size="sm"
                        shape="pill"
                        onClick={() => setStarting(true)}
                      >
                        {t("upgrade.start")}
                      </Button>
                    )}
                  </>
                )
              : undefined}
          </SettingsRow>
          {upgrade !== null && (
            <p className="-mt-1 px-4.5 pb-3.5 text-sm">
              {t(upgradeStateText[upgrade.state])}
              {admin && (
                <>
                  {" "}
                  {/* The settings left for the upgrade's page, arrived at (M13/P3/S2, decision 839). */}
                  <Link to={`${home}/upgrade`} state={arrived} className="font-medium underline underline-offset-4">
                    {t("upgrade.view")}
                  </Link>
                </>
              )}
            </p>
          )}
        </div>
      </SettingsCard>
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
            <p>
              {underRaw === 1
                ? t("upgrade.startRawOne")
                : t("upgrade.startRaw", { count: new Intl.NumberFormat(preferences.locale).format(underRaw) })}
            </p>
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
        explain={(error) => pagesLocked(error, t, me.id, (id) => distinctName(pages.tree, id, t))}
        texts={{ "transfer.busy": "transfer.writingBusy" }}
        tone="default"
      />
    </SettingsSection>
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
 * DangerSection deletes the notebook for every member, once its name is
 * typed (M3 design 4), a row of the danger zone (M13/P3/S2, decision 826);
 * the shell then goes to the workspace's home, arrived at.
 */
function DangerSection({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  return (
    <SettingsSection title={t("settings.danger")}>
      <SettingsCard>
        <SettingsRow title={t("notebookSettings.deleteTitle")} description={t("notebookSettings.deleteBody")}>
          {() => (
            <ConfirmDialog
              trigger={
                <Button variant="destructive" size="sm" shape="pill">
                  {t("notebookSettings.delete")}
                </Button>
              }
              title={t("notebookSettings.deleteConfirmTitle", { name: notebook.name })}
              description={t("notebookSettings.deleteBody")}
              typedConfirmation={{
                label: t("notebookSettings.typeName", { name: notebook.name }),
                value: notebook.name,
              }}
              confirmLabel={t("notebookSettings.deleteConfirm")}
              sendingLabel={t("notebookSettings.deleting")}
              cancelLabel={t("notebookSettings.cancel")}
              confirm={() => notebooks.remove(notebook.id)}
            />
          )}
        </SettingsRow>
      </SettingsCard>
    </SettingsSection>
  );
}
