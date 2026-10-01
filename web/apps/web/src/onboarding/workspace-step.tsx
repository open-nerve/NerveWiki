import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState, type FormEvent } from "react";
import useSWR from "swr";

import { CreateWorkspaceForm } from "../app/create-workspace-form";
import { useForm } from "../app/form";
import { NotLoaded } from "../app/not-loaded";
import { errorText } from "../app/problem-messages";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { useStore, useWorkspaces } from "../stores/context";

type StepProps = { complete: () => Promise<void> };

/**
 * WorkspaceStep sees the account into a workspace (M2/P5 design 3.7). One
 * that has one, joined by an invitation or created here, goes on at once.
 * One without creates one; while this server's creation is off, it reads
 * how to get into one, and goes on. The step's record is only that: the
 * workspaces' operations check the memberships themselves.
 */
export const WorkspaceStep = observer(function WorkspaceStep({ complete }: StepProps) {
  const workspaces = useWorkspaces();
  const { instance } = useStore();
  const t = useT();
  const list = useSWR("workspaces", () => workspaces.load());
  const info = useSWR("instance", () => instance.load());
  if (workspaces.list === undefined) {
    return <NotLoaded error={list.error} retry={() => void list.mutate()} />;
  }
  if (workspaces.list.length > 0) {
    return <GoOn complete={complete} />;
  }
  if (instance.info === undefined) {
    return <NotLoaded error={info.error} retry={() => void info.mutate()} />;
  }
  if (!instance.info.workspace_creation_enabled) {
    return <Waiting complete={complete} />;
  }
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">{t("onboarding.workspace.hint")}</p>
      {/* Once created, the account has a workspace: the step goes on as above. */}
      <CreateWorkspaceForm wide submitLabel={t("onboarding.workspace.create")} onCreated={() => {}} />
    </div>
  );
});

/**
 * GoOn completes the step as it shows, once: its effect runs again with
 * each new complete, and twice in development, which the ref keeps to the
 * first. A failure says why, with a way to try again.
 */
function GoOn({ complete }: StepProps) {
  const t = useT();
  const [failure, setFailure] = useState<unknown>();
  const started = useRef(false);

  function retry() {
    setFailure(undefined);
    complete().catch(setFailure);
  }

  useEffect(() => {
    if (!started.current) {
      started.current = true;
      complete().catch(setFailure);
    }
  }, [complete]);

  const failed = failure === undefined ? undefined : errorText(failure, t);
  if (failed === undefined) {
    return <output className="block text-muted-foreground">{t("onboarding.workspace.goingOn")}</output>;
  }
  return (
    <div className="space-y-3">
      <Alert>{failed}</Alert>
      <Button variant="outline" onClick={retry}>
        {t("status.retry")}
      </Button>
    </div>
  );
}

/** Waiting says how to get into a workspace while this server's creation is off; Continue completes the step. */
function Waiting({ complete }: StepProps) {
  const t = useT();
  const { ref, sending, banner, submit } = useForm([]);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submit({}, complete);
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <p className="text-muted-foreground">{t("createWorkspace.off")}</p>
      <Button type="submit" className="w-full" disabled={sending}>
        {t("onboarding.continue")}
      </Button>
    </form>
  );
}
