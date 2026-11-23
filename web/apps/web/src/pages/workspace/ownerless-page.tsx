import { observer } from "mobx-react-lite";
import { useState } from "react";
import { Link } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { arrived } from "../../app/arrival";
import { useFollowRole } from "../../app/follow-role";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { cardTextClass, SettingsCard, SettingsSection, useSettingsHeading } from "../../app/settings-page";
import { Alert } from "../../components/ui/alert";
import { useT } from "../../i18n/i18n";
import { twinsOf } from "../../lib/twins";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks, useOwnerless } from "../../stores/context";
import { watchReader } from "../page/readers-input";
import { AuditSection } from "./audit-section";
import { OwnerlessRow, twinKey } from "./ownerless-row";
import { useWorkspace } from "./workspace-layout";

/**
 * OwnerlessPage is a workspace's notebooks without an admin, and the audit
 * of what was done with them (M3/P5 design 3.3): its admins' alone. Anyone
 * else reads that much, and nothing is asked of the server. The list a
 * card's, untitled (the page's name says it; M13/P3/S2, decision 851), the
 * audit a group of its own.
 */
export const OwnerlessPage = observer(function OwnerlessPage() {
  const workspace = useWorkspace();
  const t = useT();
  if (workspace.role !== "admin") {
    return <p className="text-muted-foreground">{t("ownerless.adminsOnly")}</p>;
  }
  return (
    <>
      <OwnerlessSection workspace={workspace} />
      <AuditSection workspace={workspace} />
    </>
  );
});

/** The texts of a refusal on this page: a notebook ownerless no more. */
const texts = { "notebook.not_found": "ownerless.gone" } as const;

/**
 * OwnerlessSection lists the ownerless notebooks, the longest ownerless
 * first. Taking one over puts it among the account's notebooks, and says
 * so with a way to it, which leaves the settings, arrived at (M13/P3/S2,
 * decision 839); a row that leaves gives the focus to the page's h1, its
 * name (decision 815),
 * from where the reader pressed Take over, from nothing or from itself,
 * the reader having done nothing meanwhile (v0.1 design 13.2, item 26;
 * M12/P2/S1, 22.17).
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
  const heading = useSettingsHeading();
  const failed = failure === undefined ? undefined : errorText(failure, t, texts);

  function refused(refusal: unknown) {
    setFailure(refusal);
    void mutate();
    if (refusal instanceof ApiError && refusal.code === "notebook.not_found") {
      void reload(["notebook-audit", workspace.id]);
      void reload("workspaces");
    }
  }

  // The heading takes the focus from where the reader pressed, from nothing, or from itself, the reader having done
  // nothing meanwhile: one typing in the search box as the answer comes keeps the focus and the keys there.
  function toHeading(from: Element | null, reader: ReturnType<typeof watchReader>): void {
    const at = document.activeElement;
    if (!reader.acted() && (at === from || at === null || at === document.body || at === heading.current)) {
      heading.current?.focus();
    }
  }

  async function takeOver(id: string) {
    setFailure(undefined);
    setTaken(undefined);
    const from = document.activeElement;
    const reader = watchReader({ on: from === document.body ? null : from });
    try {
      const notebook = await ownerless.takeOver(id);
      notebooks.receive(notebook);
      setTaken(notebook);
      toHeading(from, reader);
      void reload(["notebook-audit", workspace.id]);
    } catch (refusal) {
      refused(refusal);
      if (refusal instanceof ApiError && refusal.code === "notebook.not_found") {
        toHeading(from, reader);
      }
    } finally {
      reader.end();
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

  const twins = twinsOf(ownerless.list ?? [], twinKey);
  return (
    <SettingsSection>
      <p className="max-w-2xl px-0.5 text-sm text-muted-foreground">{t("ownerless.body")}</p>
      {failed !== undefined && <Alert>{failed}</Alert>}
      <output className="block px-0.5 text-sm">
        {taken !== undefined && (
          <>
            {t("ownerless.takenOver", { name: taken.name })}{" "}
            <Link
              to={`/${workspace.slug}/notebooks/${taken.id}`}
              state={arrived}
              className="underline underline-offset-4"
            >
              {t("ownerless.open")}
            </Link>
          </>
        )}
      </output>
      {ownerless.list === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : ownerless.list.length === 0 ? (
        <SettingsCard>
          <p className={cardTextClass}>{t("ownerless.none")}</p>
        </SettingsCard>
      ) : (
        <SettingsCard list label={t("workspaceSettings.ownerless")}>
          {ownerless.list.map((notebook) => (
            <OwnerlessRow
              key={notebook.id}
              notebook={notebook}
              twin={twins.has(notebook.id)}
              takeOver={() => takeOver(notebook.id)}
              remove={() => remove(notebook.id)}
              removed={() => heading.current?.focus()}
            />
          ))}
        </SettingsCard>
      )}
    </SettingsSection>
  );
});
