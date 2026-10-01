import { observer } from "mobx-react-lite";
import { useRef, useState } from "react";
import { Link } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { useFollowRole } from "../../app/follow-role";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { useT } from "../../i18n/i18n";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks, useOwnerless } from "../../stores/context";
import { AuditSection } from "./audit-section";
import { OwnerlessRow } from "./ownerless-row";
import { useWorkspace } from "./workspace-layout";

/**
 * OwnerlessPage is a workspace's notebooks without an admin, and the audit
 * of what was done with them (M3/P5 design 3.3): its admins' alone. Anyone
 * else reads that much, and nothing is asked of the server.
 */
export const OwnerlessPage = observer(function OwnerlessPage() {
  const workspace = useWorkspace();
  const t = useT();
  if (workspace.role !== "admin") {
    return <p className="text-muted-foreground">{t("ownerless.adminsOnly")}</p>;
  }
  return (
    <div className="space-y-10">
      <OwnerlessSection workspace={workspace} />
      <AuditSection workspace={workspace} />
    </div>
  );
});

/** The texts of a refusal on this page: a notebook ownerless no more. */
const texts = { "notebook.not_found": "ownerless.gone" } as const;

/**
 * OwnerlessSection lists the ownerless notebooks, the longest ownerless
 * first. Taking one over puts it among the account's notebooks, and says
 * so with a way to it; a row that leaves gives the focus to the heading.
 * A refusal says why above the list, which is read again. The server
 * answers not found both for a notebook ownerless no more, which has left
 * the list, and to an account no longer the workspace's admin: the audit,
 * which has what another did, and the workspaces, whose roles the page
 * follows, are read again too (M3/P5 review M1).
 */
const OwnerlessSection = observer(function OwnerlessSection({ workspace }: { workspace: Workspace }) {
  const ownerless = useOwnerless(workspace);
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const { mutate: reload } = useSWRConfig();
  const { error, mutate } = useSWR(["ownerless", workspace.id], () => ownerless.load(), {
    onError: useFollowRole(),
  });
  const [failure, setFailure] = useState<unknown>();
  const [taken, setTaken] = useState<Notebook>();
  const heading = useRef<HTMLHeadingElement>(null);
  const failed = failure === undefined ? undefined : errorText(failure, t, texts);

  function refused(refusal: unknown) {
    setFailure(refusal);
    void mutate();
    if (refusal instanceof ApiError && refusal.code === "notebook.not_found") {
      void reload(["notebook-audit", workspace.id]);
      void reload("workspaces");
      heading.current?.focus();
    }
  }

  async function takeOver(id: string) {
    setFailure(undefined);
    setTaken(undefined);
    try {
      const notebook = await ownerless.takeOver(id);
      notebooks.receive(notebook);
      setTaken(notebook);
      heading.current?.focus();
      void reload(["notebook-audit", workspace.id]);
    } catch (refusal) {
      refused(refusal);
    }
  }

  /** remove deletes the notebook id; one ownerless no more is said above the list, and the dialog closes. */
  async function remove(id: string) {
    setFailure(undefined);
    setTaken(undefined);
    try {
      await ownerless.remove(id);
      void reload(["notebook-audit", workspace.id]);
    } catch (refusal) {
      if (!(refusal instanceof ApiError && refusal.code === "notebook.not_found")) {
        throw refusal;
      }
      refused(refusal);
    }
  }

  return (
    <section className="space-y-4">
      <div className="max-w-2xl space-y-1">
        <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">
          {t("workspaceSettings.ownerless")}
        </h2>
        <p className="text-sm text-muted-foreground">{t("ownerless.body")}</p>
      </div>
      {failed !== undefined && <Alert>{failed}</Alert>}
      <output className="block text-sm">
        {taken !== undefined && (
          <>
            {t("ownerless.takenOver", { name: taken.name })}{" "}
            <Link to={`/${workspace.slug}/notebooks/${taken.id}`} className="underline underline-offset-4">
              {t("ownerless.open")}
            </Link>
          </>
        )}
      </output>
      {ownerless.list === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : ownerless.list.length === 0 ? (
        <p className="text-muted-foreground">{t("ownerless.none")}</p>
      ) : (
        <ul aria-label={t("workspaceSettings.ownerless")} className="divide-y rounded-md border">
          {ownerless.list.map((notebook) => (
            <OwnerlessRow
              key={notebook.id}
              notebook={notebook}
              takeOver={() => takeOver(notebook.id)}
              remove={() => remove(notebook.id)}
              removed={() => heading.current?.focus()}
            />
          ))}
        </ul>
      )}
    </section>
  );
});
