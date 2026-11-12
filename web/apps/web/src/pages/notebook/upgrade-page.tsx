import { observer } from "mobx-react-lite";
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode, type RefObject } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import useSWR, { SWRConfig, unstable_serialize, useSWRConfig } from "swr";

import { arrived, useArrivalFocus } from "../../app/arrival";
import { ConfirmDialog } from "../../app/confirm-dialog";
import { useDocumentTitle } from "../../app/document-title";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { Button } from "../../components/ui/button";
import { formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { Upgrade } from "../../services/upgrade.service";
import type { Workspace } from "../../services/workspace.service";
import { useAccount, useNotebooks, usePageTree, useStore, useTransfers, useUpgrades } from "../../stores/context";
import { under } from "../../stores/transfer.store";
import { nodeTarget } from "../node-target";
import { useWorkspace } from "../workspace/workspace-layout";
import { distinctName } from "./distinct-name";
import { useNotebook } from "./notebook-layout";
import { TransferJobRow } from "./transfer-job-row";
import { jobNames } from "./transfer-names";
import { UpgradeCheckSection, UpgradeReportSection, type CheckControl, type TreeRead } from "./upgrade-check";

/**
 * UpgradePage is the notebook's upgrade to an LLM Wiki under way (M11/P1
 * design 4.4), its admins' alone (UpgradeRead). Its title takes the focus
 * when the page is arrived at, from the start of the upgrade, and once a
 * change asked here is done. Each arrival is an entry of its own, a link
 * to the page followed on it too (M11/P2 design 4.3, as the health page's).
 */
export const UpgradePage = observer(function UpgradePage() {
  const notebook = useNotebook();
  const workspace = useWorkspace();
  const location = useLocation();
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  useDocumentTitle(t("upgrade.title"), notebook.name);
  return (
    <div className="max-w-2xl space-y-8">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
        {t("upgrade.title")}
      </h1>
      <UpgradeRead
        key={location.key}
        notebook={notebook}
        workspace={workspace}
        heading={heading}
        locationKey={location.key}
      />
    </div>
  );
});

let nextCheckEntry = 0;

/**
 * The changes asked on the page: abandoning the upgrade, continuing its
 * stopped rollback, ending it; submitting it for review (M11/P2);
 * approving it, sending it back, rejecting it (M11/P3).
 */
type Change = "abandon" | "continue" | "end" | "submit" | "approve" | "sendBack" | "reject";

/**
 * How long after a send-back refused while the submission is still
 * checked the upgrade is read once more: the bound of a submission's
 * check, LintTimeout plus 10 seconds, LintTimeout at most 10 seconds (M11
 * design 4.15); the page does not know the instance's.
 */
const sendBackWaitMs = 20_000;

/**
 * UpgradeRead reads the upgrade (SWR's key ["upgrade", notebook id], which
 * the mode event and each connection read again), for the notebook's
 * admins alone: to the others it says who can see it and reads nothing
 * (v0.1 design 13.2, item 18). Refused as forbidden, the account is no
 * longer the notebook's admin, and the workspace's notebooks are read
 * again, whose roles the page follows; none under way says so. It holds
 * the dialogs of the changes, which stay, with their refusals, when a read
 * again takes their buttons, the upgrade, or the account's role away:
 * closed, the focus goes back to the button, or to the title once it is
 * gone or the change is done.
 *
 * Upgrading, it offers to submit the upgrade for review (decision 259):
 * once its first step is done, the upgrade is reviewing, as the mode event
 * reads it again, its check gone, the dialog staying until the answer
 * (decision 264); refused as not ready or of an old index, the upgrade is
 * upgrading again, read again, and checked again. Reviewing, it shows the
 * report the submission kept, and offers to approve it, once it has its
 * report, to send it back for more work, and to reject it, in place of
 * abandoning it (decisions 288, 291; M11/P3 design 4.3). Approved, the
 * upgrade is no longer under way: the page, still there, goes to the
 * notebook's home, whose title takes the focus (decision 289), and the
 * upgrade, in review, is dropped from the cache. Sent back, by this
 * admin or another, the check of the submission's time is not shown
 * again (decision 300); refused while the submission is still checked,
 * as the upgrade read again after the refusal says, the dialog says so
 * until it closes, and that submission, still checked, is read once more
 * after sendBackWaitMs.
 *
 * It is one entry to the page (M11/P2 design 4.3): the check it shows is
 * of its generation, which a new check moves on, of the upgrade it was
 * asked for (another upgrade shown in the entry is not checked until
 * Check is pressed), in a cache of its own that wraps the check and the
 * report (M11/P3 design 4.3), mounted as long as the entry, so that the
 * check gone and back finds its read; leaving the page aborts the
 * requests of the check and of the report's cards and drops that cache.
 * The upgrade, the notebooks and the tree are read in the public
 * provider, as the left column's; the tree failed to read, the sections
 * say so above their nodes, and read it again when asked.
 */
const UpgradeRead = observer(function UpgradeRead({
  notebook,
  workspace,
  heading,
  locationKey,
}: {
  notebook: Notebook;
  workspace: Workspace;
  heading: RefObject<HTMLHeadingElement | null>;
  locationKey: string;
}) {
  const upgrades = useUpgrades();
  const notebooks = useNotebooks(workspace);
  const { preferences } = useStore();
  const { cache, mutate } = useSWRConfig();
  const mounted = useMounted();
  const t = useT();
  const pages = usePageTree(notebook);
  const [nonce] = useState(() => ++nextCheckEntry);
  const control = useRef<CheckControl>({ alive: true, generation: 0, controllers: new Set() });
  const entry = useMemo(() => ({ nonce, control, locationKey }), [nonce, locationKey]);
  const [checked, setChecked] = useState({ generation: 0, upgrade: "" });
  const [checks] = useState(() => new Map());
  const [checksConfig] = useState(() => ({ provider: () => checks }));
  useLayoutEffect(() => {
    const owner = control.current;
    owner.alive = true;
    return () => {
      owner.alive = false;
      for (const controller of owner.controllers) controller.abort();
      owner.controllers.clear();
      checks.clear();
    };
  }, [checks]);
  /**
   * recheck checks the upgrade upgrade again: the entry's next generation,
   * of that upgrade, read once the check is shown, its last one's requests
   * aborted. Another upgrade the entry shows later has none until checked.
   */
  function recheck(upgrade: string) {
    const owner = control.current;
    if (!owner.alive) return;
    owner.generation += 1;
    for (const controller of owner.controllers) controller.abort();
    owner.controllers.clear();
    setChecked({ generation: owner.generation, upgrade });
  }
  const [asked, setAsked] = useState<Change | undefined>(undefined);
  const buttons = {
    abandon: useRef<HTMLButtonElement>(null),
    continue: useRef<HTMLButtonElement>(null),
    end: useRef<HTMLButtonElement>(null),
    submit: useRef<HTMLButtonElement>(null),
    approve: useRef<HTMLButtonElement>(null),
    sendBack: useRef<HTMLButtonElement>(null),
    reject: useRef<HTMLButtonElement>(null),
  };
  const navigate = useNavigate();
  const admin = notebook.role === "admin";
  const rereadNotebooks = () => void mutate(["notebooks", workspace.id]);
  const {
    data,
    error,
    mutate: reread,
  } = useSWR(admin ? ["upgrade", notebook.id] : null, () => upgrades.get(notebook.id), {
    onError: (refusal: unknown) => {
      if (refusal instanceof ApiError && refusal.code === "forbidden") {
        rereadNotebooks();
      }
    },
  });
  const tree = useSWR(admin ? ["pages", notebook.id] : null, () => pages.load());
  const treeRead: TreeRead = {
    error: tree.error,
    reading: (pages.nodes === undefined && tree.error === undefined) || tree.isValidating,
    retry: () => void tree.mutate(),
  };

  /**
   * change sends what a dialog asks, then reads the upgrade again; so does
   * a refusal that says the page is behind: a rollback under way already,
   * the upgrade in another state, or upgrading again after a submission
   * not ready or of an old index, read before the refusal is thrown, so
   * that what the dialog says of it is of the upgrade as it is read
   * again. The upgrade an abandonment, a submission or a
   * continuation answers, rolling back, is shown meanwhile (shows), over
   * what the page showed as it was sent, before the read again (SWR keeps
   * no read sent before it); a read that answered meanwhile, a mode
   * event's, is newer and stays. An end's answer, ended, is not shown: the
   * read says none is under way. The answer is shown even once the page is
   * left, which shows it when it is come back to.
   */
  async function change(send: () => Promise<Upgrade>, shows: boolean): Promise<void> {
    const key = unstable_serialize(["upgrade", notebook.id]);
    const sent = cache.get(key);
    let answered: Upgrade;
    try {
      answered = await send();
    } catch (refusal) {
      if (
        mounted() &&
        refusal instanceof ApiError &&
        (refusal.code === "transfer.busy" ||
          refusal.code === "wiki.upgrade_state" ||
          refusal.code === "wiki.upgrade_not_ready" ||
          refusal.code === "wiki.upgrade_stale_index")
      ) {
        await reread();
      }
      throw refusal;
    }
    // Its data and error, not the whole state, which a read under way replaces too. A function of the data would not
    // do: SWR clears the error of whatever a mutate writes, a 404 a read answered meanwhile included.
    const now = cache.get(key);
    if (shows && now?.data === sent?.data && now?.error === sent?.error) {
      void mutate(["upgrade", notebook.id], answered, { revalidate: false });
    }
    if (mounted()) {
      void reread();
    }
  }

  /**
   * submit submits the upgrade for review as change does; refused as not
   * ready or of an old index, the upgrade, upgrading again, is checked
   * again (M11 design 4.8): the next generation, read once its check is
   * shown, as the page reads the upgrade again.
   */
  async function submit(): Promise<void> {
    const submitted = data?.id;
    try {
      await change(() => notebooks.submitUpgrade(notebook.id), true);
    } catch (refusal) {
      if (
        refusal instanceof ApiError &&
        (refusal.code === "wiki.upgrade_not_ready" || refusal.code === "wiki.upgrade_stale_index") &&
        submitted !== undefined
      ) {
        recheck(submitted);
      }
      throw refusal;
    }
  }

  // Approved, the page goes: unmounted, the upgrade in review leaves the cache.
  const approvedAway = useRef(false);
  useEffect(
    () => () => {
      if (approvedAway.current) void mutate(["upgrade", notebook.id], undefined, { revalidate: false });
    },
    [mutate, notebook.id]
  );

  /**
   * approve approves the upgrade (decision 289; design review P3C-M2):
   * refused for its state, or a gate checked again not passing, the
   * upgrade is read again, as change does; approved, it is no longer under
   * way, its read answering 404, and nothing is read again nor shown over
   * it: the page, still there, goes to the notebook's home, its title
   * taking the focus; once it is unmounted, the upgrade, in review, is
   * dropped from the cache, read by no hook then, so that Back does not
   * show it meanwhile (cleared before, the page would show it loading as
   * it goes).
   */
  async function approve(): Promise<void> {
    try {
      await notebooks.approveUpgrade(notebook.id);
    } catch (refusal) {
      if (
        mounted() &&
        refusal instanceof ApiError &&
        (refusal.code === "wiki.upgrade_state" || refusal.code === "wiki.upgrade_not_ready")
      ) {
        void reread();
      }
      throw refusal;
    }
    if (mounted()) {
      approvedAway.current = true;
      await navigate(`/${workspace.slug}/notebooks/${notebook.id}`, { state: arrived });
    }
  }

  // The last send-back's refusal while its submission was still checked, as the upgrade read again after it says.
  const [stillChecked, setStillChecked] = useState<ApiError | undefined>(undefined);
  // The submission whose check the page waits for, to read the upgrade once more; none once it has.
  const [waiting, setWaiting] = useState<string | undefined>(undefined);
  // That submission is shown in review without a report: still checked, or it did not finish.
  const due =
    error === undefined &&
    data?.state === "reviewing" &&
    data.report === null &&
    waiting !== undefined &&
    data.submitted_at === waiting;
  useEffect(() => {
    if (!due) {
      return undefined;
    }
    const timer = setTimeout(() => {
      setWaiting(undefined);
      void reread();
    }, sendBackWaitMs);
    return () => clearTimeout(timer);
  }, [due, reread]);

  // What the page showed of the upgrade's review (decision 300): upgrading again once its report was shown, or another
  // submission was, it was sent back, by this admin or another, and the check of the submission's time is not shown
  // again; upgrading again after a submission refused as not ready or of an old index, its check is the one asked then.
  const review = useRef<{ submitted: string | null; reported: boolean; changed: boolean } | undefined>(undefined);
  useEffect(() => {
    if (data === undefined || error !== undefined) {
      return;
    }
    const was = review.current;
    if (data.state === "reviewing") {
      review.current = {
        submitted: data.submitted_at,
        reported: (was?.reported ?? false) || data.report !== null,
        changed: (was?.changed ?? false) || (was !== undefined && was.submitted !== data.submitted_at),
      };
      return;
    }
    review.current = undefined;
    if (data.state === "upgrading" && was !== undefined && (was.reported || was.changed)) {
      setChecked({ generation: 0, upgrade: "" });
    }
  }, [data, error]);

  /**
   * sendBack sends the upgrade back, as change does (M11/P3 design 4.3):
   * done, upgrading again, the check of the entry is none again, Check to
   * be pressed (design review P3C-N15). Refused for its state, and the
   * upgrade, as change read it again, in review without a report, its
   * submission is still checked: the dialog says so of this refusal, what
   * is read after it notwithstanding (design review P3C-N14), and that
   * submission is read once more after sendBackWaitMs, unless it is no
   * longer shown in review without a report, the page goes, or it is sent
   * back again first. A read again that failed says nothing of it; one
   * overtaken by a newer read, its answer dropped, is read once more.
   */
  async function sendBack(): Promise<void> {
    setStillChecked(undefined);
    setWaiting(undefined);
    try {
      await change(() => notebooks.sendBackUpgrade(notebook.id), true);
    } catch (refusal) {
      const key = unstable_serialize(["upgrade", notebook.id]);
      if (mounted() && cache.get(key)?.isValidating === true) {
        // change's read again was overtaken by a newer one, a mode event's: its answer dropped, the cache is as before.
        await reread();
      }
      const read = cache.get(key);
      const now = read?.data as Upgrade | undefined;
      if (
        mounted() &&
        refusal instanceof ApiError &&
        refusal.code === "wiki.upgrade_state" &&
        read?.error === undefined &&
        now?.state === "reviewing" &&
        now.report === null &&
        now.submitted_at !== null
      ) {
        setStillChecked(refusal);
        setWaiting(now.submitted_at);
      }
      throw refusal;
    }
    setChecked({ generation: 0, upgrade: "" });
  }

  function dialogOf(which: Change) {
    return {
      open: asked === which,
      onOpenChange: (open: boolean) => setAsked(open ? which : undefined),
      onClosed: (done: boolean) => (done ? heading.current : (buttons[which].current ?? heading.current))?.focus(),
    };
  }

  let shown: ReactNode;
  // The check, upgrading, or the report, reviewing: this entry's own, in its cache.
  let entryOwn: ReactNode = null;
  let rest: ReactNode = null;
  if (!admin) {
    shown = <p>{t("upgrade.adminsOnly")}</p>;
  } else if (error instanceof ApiError && error.code === "wiki.upgrade_not_found") {
    shown = <p>{t("problem.wiki.upgrade_not_found")}</p>;
  } else if (data === undefined || error !== undefined) {
    shown = <NotLoaded error={error} retry={() => void reread()} />;
  } else {
    shown = (
      <>
        <UpgradeState notebook={notebook} upgrade={data} />
        {data.state === "upgrading" && <AgentSection upgrade={data} />}
      </>
    );
    if (data.state === "upgrading") {
      entryOwn = (
        <UpgradeCheckSection
          notebook={notebook}
          upgrade={data}
          entry={entry}
          generation={checked.upgrade === data.id ? checked.generation : 0}
          recheck={() => recheck(data.id)}
          rereadUpgrade={() => void reread()}
          rereadNotebooks={rereadNotebooks}
          pages={pages}
          tree={treeRead}
        />
      );
    } else if (data.state === "reviewing") {
      entryOwn = (
        <UpgradeReportSection
          notebook={notebook}
          upgrade={data}
          entry={entry}
          rereadUpgrade={() => void reread()}
          rereadNotebooks={rereadNotebooks}
          pages={pages}
          tree={treeRead}
        />
      );
    }
    rest = (
      <>
        {data.state === "rolling_back" && (
          <RollbackSection
            notebook={notebook}
            upgrade={data}
            rereadUpgrade={() => void reread()}
            ask={setAsked}
            continueButton={buttons.continue}
            endButton={buttons.end}
          />
        )}
        {data.state === "upgrading" && (
          <div className="flex flex-wrap gap-3">
            <Button ref={buttons.submit} onClick={() => setAsked("submit")}>
              {t("upgrade.submit")}
            </Button>
            <Button ref={buttons.abandon} variant="destructive" onClick={() => setAsked("abandon")}>
              {t("upgrade.abandon")}
            </Button>
          </div>
        )}
        {data.state === "reviewing" && (
          // Rejecting stands for abandoning in review (decision 291); approving waits for the report (decision 288).
          <div className="flex flex-wrap gap-3">
            {data.report !== null && (
              <Button ref={buttons.approve} onClick={() => setAsked("approve")}>
                {t("upgrade.approve")}
              </Button>
            )}
            <Button ref={buttons.sendBack} variant="outline" onClick={() => setAsked("sendBack")}>
              {t("upgrade.sendBack")}
            </Button>
            <Button ref={buttons.reject} variant="destructive" onClick={() => setAsked("reject")}>
              {t("upgrade.reject")}
            </Button>
          </div>
        )}
      </>
    );
  }
  const left = data?.rollback_left ?? null;
  const texts = { "transfer.busy": "upgrade.rollbackUnderWay" } as const;
  // The source area the approval makes read only, by its path, once the tree has it (design review P3C-N13).
  const sourceRoot = data === undefined ? undefined : nodeTarget(pages, data.source_root_id)?.path;
  return (
    <>
      {shown}
      <SWRConfig value={checksConfig}>{entryOwn}</SWRConfig>
      {rest}
      <ConfirmDialog
        held={dialogOf("abandon")}
        title={t("upgrade.abandonTitle", { name: notebook.name })}
        description={t("upgrade.abandonBody")}
        confirmLabel={t("upgrade.abandonConfirm")}
        sendingLabel={t("upgrade.abandoning")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={() => change(() => notebooks.abandonUpgrade(notebook.id), true)}
        texts={texts}
      />
      <ConfirmDialog
        held={dialogOf("continue")}
        title={t("upgrade.continueTitle", { name: notebook.name })}
        description={t("upgrade.continueBody")}
        confirmLabel={t("upgrade.continueConfirm")}
        sendingLabel={t("upgrade.continuing")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={() => change(() => notebooks.abandonUpgrade(notebook.id), true)}
        texts={texts}
        tone="default"
      />
      <ConfirmDialog
        held={dialogOf("end")}
        title={t("upgrade.endTitle", { name: notebook.name })}
        description={t("upgrade.endBody")}
        more={
          left === null ? undefined : left.nodes + left.pages + left.created === 0 ? (
            <p>{t("upgrade.endNothingLeft")}</p>
          ) : (
            <>
              <LeftCounts left={left} locale={preferences.locale} />
              <p className="mt-2">{t("upgrade.endLinks")}</p>
            </>
          )
        }
        confirmLabel={t("upgrade.endConfirm")}
        sendingLabel={t("upgrade.ending")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={() => change(() => notebooks.endUpgradeRollback(notebook.id), false)}
        texts={texts}
      />
      <ConfirmDialog
        held={dialogOf("submit")}
        title={t("upgrade.submitTitle")}
        description={t("upgrade.submitBody")}
        confirmLabel={t("upgrade.submitConfirm")}
        sendingLabel={t("upgrade.submitting")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={submit}
        texts={{ "wiki.upgrade_not_ready": "upgrade.notReady" }}
        tone="default"
      />
      <ConfirmDialog
        held={dialogOf("approve")}
        title={t("upgrade.approveTitle", { name: notebook.name })}
        description={t("upgrade.approveBody", { root: sourceRoot ?? t("upgrade.approveRootUnread") })}
        confirmLabel={t("upgrade.approveConfirm")}
        sendingLabel={t("upgrade.approving")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={approve}
        texts={{ "wiki.upgrade_not_ready": "upgrade.approveNotReady" }}
        tone="default"
      />
      <ConfirmDialog
        held={dialogOf("sendBack")}
        title={t("upgrade.sendBackTitle", { name: notebook.name })}
        description={t("upgrade.sendBackBody")}
        confirmLabel={t("upgrade.sendBackConfirm")}
        sendingLabel={t("upgrade.sendingBack")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={sendBack}
        // As the upgrade was read again after this refusal, not as it was when the dialog opened, nor as read since (P3C-N14).
        explain={(refusal) =>
          stillChecked !== undefined && refusal === stillChecked ? t("upgrade.sendBackChecking") : undefined
        }
        tone="default"
      />
      <ConfirmDialog
        held={dialogOf("reject")}
        title={t("upgrade.rejectTitle", { name: notebook.name })}
        description={t("upgrade.rejectBody")}
        confirmLabel={t("upgrade.rejectConfirm")}
        sendingLabel={t("upgrade.rejecting")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={() => change(() => notebooks.rejectUpgrade(notebook.id), true)}
      />
    </>
  );
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

/**
 * AgentSection tells how the upgrade's work is done while it is under way
 * (decision 248): its initiator connects their agent, over MCP, as the
 * README says, and gives it the prompt upgrade; another admin, whose agent
 * cannot write, is told whose can, and that they may abandon it. Submitted
 * or rolling back, no agent writes, and the section is not shown.
 */
function AgentSection({ upgrade }: { upgrade: Upgrade }) {
  const { me } = useAccount();
  const t = useT();
  return (
    <section className="space-y-2">
      <h2 className="text-lg font-semibold">{t("upgrade.agent")}</h2>
      {upgrade.started_by.user_id === me.id ? (
        <>
          <p>{t("upgrade.agentYours")}</p>
          <p>
            {t("upgrade.agentPrompt")} <code className="rounded bg-muted px-1 font-mono text-sm">upgrade</code>
          </p>
        </>
      ) : (
        <p>{t("upgrade.agentOthers", { name: upgrade.started_by.display_name })}</p>
      )}
    </section>
  );
}

/**
 * RollbackSection is the upgrade rolling back (M11/P1 design 4.4): its
 * job, read by its id, every second while it is queued or running, as the
 * jobs page reads its own (the rollback's progress sends no event); once
 * the job is seen ended while the upgrade read says it is under way, the
 * upgrade is read again, once for the job, which tells what the rollback
 * left, or that it ended. Stopped, it says what is left, and offers to
 * continue the rollback or to end it, keeping the notebook as it is; not
 * to abandon it, which is what continuing is (decision 248).
 */
const RollbackSection = observer(function RollbackSection({
  notebook,
  upgrade,
  rereadUpgrade,
  ask,
  continueButton,
  endButton,
}: {
  notebook: Notebook;
  upgrade: Upgrade;
  rereadUpgrade: () => void;
  ask: (change: Change) => void;
  continueButton: RefObject<HTMLButtonElement | null>;
  endButton: RefObject<HTMLButtonElement | null>;
}) {
  const { preferences } = useStore();
  const t = useT();
  const left = upgrade.rollback_left;
  return (
    <section className="space-y-3">
      <h2 className="text-lg font-semibold">{t("upgrade.rollback")}</h2>
      {upgrade.rollback_job_id !== null && (
        <RollbackJob
          notebook={notebook}
          jobId={upgrade.rollback_job_id}
          readsUpgrade={left === null}
          rereadUpgrade={rereadUpgrade}
        />
      )}
      {left !== null && (
        <>
          <p>{t("upgrade.rollbackStopped")}</p>
          <LeftCounts left={left} locale={preferences.locale} />
          <div className="flex flex-wrap gap-3">
            <Button ref={continueButton} variant="outline" onClick={() => ask("continue")}>
              {t("upgrade.continue")}
            </Button>
            <Button ref={endButton} variant="destructive" onClick={() => ask("end")}>
              {t("upgrade.end")}
            </Button>
          </div>
        </>
      )}
    </section>
  );
});

/**
 * RollbackJob is the rollback's job as the jobs page shows it, read by its
 * id (SWR's key ["transfer-job", id], the report's own): polled every
 * second while under way; ended while readsUpgrade (the upgrade read with
 * its job under way), the upgrade is read again, once for the job.
 */
const RollbackJob = observer(function RollbackJob({
  notebook,
  jobId,
  readsUpgrade,
  rereadUpgrade,
}: {
  notebook: Notebook;
  jobId: string;
  readsUpgrade: boolean;
  rereadUpgrade: () => void;
}) {
  const transfers = useTransfers(notebook);
  const { me } = useAccount();
  const { preferences } = useStore();
  const t = useT();
  const { data, error, mutate } = useSWR(["transfer-job", jobId], () => transfers.detail(jobId), {
    refreshInterval: (latest) => (latest !== undefined && under(latest) ? 1000 : 0),
    // A second's polling is not deduplicated away.
    dedupingInterval: 500,
  });
  const ended = data !== undefined && !under(data);
  const reread = useRef(new Set<string>());
  useEffect(() => {
    if (ended && readsUpgrade && !reread.current.has(jobId)) {
      reread.current.add(jobId);
      rereadUpgrade();
    }
  }, [ended, readsUpgrade, jobId, rereadUpgrade]);
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const name = jobNames([data], t, preferences.locale, me.id).get(data.id) ?? data.name;
  return (
    <ul aria-label={t("upgrade.rollbackJob")} className="rounded-md border">
      <TransferJobRow notebook={notebook} job={data} name={name} reread={() => void mutate()} />
    </ul>
  );
});

/** LeftCounts is what a stopped rollback left: the nodes not back in place, the pages not written back, the new nodes still there. */
function LeftCounts({ left, locale }: { left: NonNullable<Upgrade["rollback_left"]>; locale: Locale }) {
  const t = useT();
  const number = new Intl.NumberFormat(locale);
  const counts = [
    ["upgrade.leftNodes", left.nodes],
    ["upgrade.leftPages", left.pages],
    ["upgrade.leftCreated", left.created],
  ] as const;
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
      {counts.map(([label, count]) => (
        <div key={label} className="contents">
          <dt className="text-muted-foreground">{t(label)}</dt>
          <dd>{number.format(count)}</dd>
        </div>
      ))}
    </dl>
  );
}
