import { observer } from "mobx-react-lite";
import { useId, useRef, type Ref } from "react";
import useSWR from "swr";

import { Button } from "../../components/ui/button";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { useT, type Translate } from "../../i18n/i18n";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { shortId, twinsOf } from "../../lib/twins";
import type { Notebook } from "../../services/notebook.service";
import type { Changeset, LogFilter } from "../../services/page.service";
import { useMembers, usePageTree } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { clientText } from "./changeset-text";

/** The clients a history is always filtered by: each of its own, and every MCP client. */
const clients = ["web", "api", "cli", "mcp"];

/**
 * An account a history may be filtered by: its id, its name, and what tells it from another of its name, which twin
 * says another listed has.
 */
export type Actor = { id: string; name: string; detail: string; twin: boolean };

/**
 * actorsOf are the accounts a history may be filtered by (M8/P3 design
 * 4.2): the workspace's members, when they could be read, with the email
 * each has, and those of the changesets read that are not among them any
 * more; one the address names that neither has, by the end of its id.
 * Those of one name are told apart by their email, or the end of their
 * ids (v0.1 design 13.2, item 17).
 */
function actorsOf(
  members: readonly { user_id: string; display_name: string; email: string | null }[],
  read: readonly Changeset[],
  chosen: string,
  t: Translate
): Actor[] {
  const actors = new Map<string, Actor>();
  for (const member of members) {
    actors.set(member.user_id, {
      id: member.user_id,
      name: member.display_name,
      detail: member.email ?? t("history.id", { id: shortId(member.user_id) }),
      twin: false,
    });
  }
  for (const { created_by: by } of read) {
    if (!actors.has(by.user_id)) {
      actors.set(by.user_id, {
        id: by.user_id,
        name: by.display_name,
        detail: t("history.id", { id: shortId(by.user_id) }),
        twin: false,
      });
    }
  }
  if (chosen !== "" && !actors.has(chosen)) {
    actors.set(chosen, { id: chosen, name: t("history.someone", { id: shortId(chosen) }), detail: "", twin: false });
  }
  const list = [...actors.values()];
  const twins = twinsOf(list, (actor) => actor.name);
  for (const actor of list) {
    actor.twin = twins.has(actor.id);
  }
  return list;
}

/**
 * useActors are the accounts a history may be filtered by (actorsOf): the
 * workspace's members as its pages read them, none where they cannot be,
 * those of the changesets read (read), and the one chosen.
 */
export function useActors(read: readonly Changeset[], chosen: string): Actor[] {
  const t = useT();
  const workspace = useWorkspace();
  const members = useMembers(workspace);
  useSWR(["members", workspace.id], () => members.load());
  return actorsOf(members.list ?? [], read, chosen, t);
}

/**
 * HistoryFilters narrow a notebook's history (M8/P3 design 4.2): who made
 * the changesets, of the accounts read (actors, useActors), whose choice
 * holds whoRef; where they came from, each client and every MCP client, and
 * the MCP clients of those read; and the page they changed, which comes
 * from a page's menu, a changeset's page or a revert's conflicts alone,
 * said as it is now, or as deleted where the tree has it not, with a way
 * to take it off. A choice is the address's (choose), which the list
 * follows; the focus stays on what chose it. Remove, gone with the focus,
 * gives it away (left).
 */
export const HistoryFilters = observer(function HistoryFilters({
  notebook,
  filter,
  read,
  actors,
  whoRef,
  choose,
  left,
}: {
  notebook: Notebook;
  filter: LogFilter;
  read: readonly Changeset[];
  actors: readonly Actor[];
  whoRef: Ref<HTMLSelectElement>;
  choose: (filter: LogFilter) => void;
  left: () => void;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const ids = { actor: useId(), client: useId() };
  const mcps = read.map((changeset) => changeset.client).filter((client) => client.startsWith("mcp:"));
  const shown = [...new Set([...clients, ...mcps, ...(filter.client === "" ? [] : [filter.client])])];
  const node = filter.page === "" ? undefined : pages.nodes?.find((each) => each.id === filter.page);
  const page = node?.name ?? (pages.nodes === undefined ? "…" : t("history.pageDeleted"));
  return (
    <div className="flex flex-wrap items-end gap-4">
      <div className="space-y-2">
        <Label htmlFor={ids.actor}>{t("history.actor")}</Label>
        <NativeSelect
          ref={whoRef}
          id={ids.actor}
          value={filter.actor}
          onChange={(event) => choose({ ...filter, actor: event.target.value })}
        >
          <option value="">{t("history.everyone")}</option>
          {actors.map((actor) => (
            <option key={actor.id} value={actor.id}>
              {actor.twin ? t("history.actorTwin", { name: actor.name, detail: actor.detail }) : actor.name}
            </option>
          ))}
        </NativeSelect>
      </div>
      <div className="space-y-2">
        <Label htmlFor={ids.client}>{t("history.client")}</Label>
        <NativeSelect
          id={ids.client}
          value={filter.client}
          onChange={(event) => choose({ ...filter, client: event.target.value })}
        >
          <option value="">{t("history.everyClient")}</option>
          {shown.map((client) => (
            <option key={client} value={client}>
              {clientText(client, t)}
            </option>
          ))}
        </NativeSelect>
      </div>
      {filter.page !== "" && <PageFilter name={page} remove={() => choose({ ...filter, page: "" })} left={left} />}
    </div>
  );
});

/**
 * PageFilter is the page the history is filtered by, by name, with Remove
 * (remove), which, gone with the focus, gives it away (left).
 */
function PageFilter({ name, remove, left }: { name: string; remove: () => void; left: () => void }) {
  const t = useT();
  const button = useRef<HTMLButtonElement>(null);
  useFocusLeaving(button, left);
  return (
    <p className="flex h-9 items-center gap-2 rounded-md border px-3 text-sm">
      <span className="break-words">{t("history.page", { name })}</span>
      <Button
        ref={button}
        variant="ghost"
        className="h-7 px-2 text-xs"
        aria-label={t("history.removePageLabel", { name })}
        onClick={remove}
      >
        {t("history.removePage")}
      </Button>
    </p>
  );
}
