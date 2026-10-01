import { observer } from "mobx-react-lite";
import { useNavigate } from "react-router";
import useSWR from "swr";

import { CreateWorkspaceForm } from "../app/create-workspace-form";
import { NotLoaded } from "../app/not-loaded";
import { useT } from "../i18n/i18n";
import { useStore } from "../stores/context";

/**
 * CreateWorkspacePage creates a workspace, of which the account becomes the
 * admin, and goes into it (M2/P5 design 3.5). While this server's creation
 * is off, it says how one gets into a workspace instead: it is also where /
 * lands an account without one.
 */
export const CreateWorkspacePage = observer(function CreateWorkspacePage() {
  const { instance } = useStore();
  const t = useT();
  const navigate = useNavigate();
  const { error, mutate } = useSWR("instance", () => instance.load());
  const info = instance.info;
  return (
    <section className="mx-auto max-w-sm space-y-6">
      <h1 className="text-2xl font-semibold">{t("createWorkspace.title")}</h1>
      {info === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : info.workspace_creation_enabled ? (
        <CreateWorkspaceForm
          submitLabel={t("createWorkspace.submit")}
          onCreated={(workspace) => void navigate(`/${workspace.slug}`)}
        />
      ) : (
        <p className="text-muted-foreground">{t("createWorkspace.off")}</p>
      )}
    </section>
  );
});
