import { observer } from "mobx-react-lite";

import { useArrivalFocus } from "../../app/arrival";
import { useT } from "../../i18n/i18n";
import { useWorkspace } from "./workspace-layout";

/** WorkspaceHomePage is a workspace's first page: M3 lists its notebooks here. */
export const WorkspaceHomePage = observer(function WorkspaceHomePage() {
  const workspace = useWorkspace();
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  return (
    <section className="space-y-2">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
        {workspace.name}
      </h1>
      <p className="text-muted-foreground">{t("workspace.homeEmpty")}</p>
    </section>
  );
});
