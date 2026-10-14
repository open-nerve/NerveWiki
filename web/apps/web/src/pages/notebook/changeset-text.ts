import { formatDateTime } from "../../i18n/format";
import type { Translate } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import { shortId, twinsOf } from "../../lib/twins";
import type { Changeset, ChangesetBrief, ChangesetChange, RevertConflict } from "../../services/page.service";

// What the history says of a changeset (M8 design 4.7, 4.13; M8/P3 design
// 4.2, 4.3): its kind, its client, its name, its counts and its changes,
// which the log's rows, a changeset's page and a version's page share; and
// why a node keeps its revert, which the revert's dialog and a revert job's
// report share (M8/P4 design 4.3).

/** kindText is the name of a changeset's kind, every kind the contract has. */
export function kindText(kind: Changeset["kind"], t: Translate): string {
  switch (kind) {
    case "edit":
      return t("history.kind.edit");
    case "import":
      return t("history.kind.import");
    case "version_restore":
      return t("history.kind.version_restore");
    case "trash_restore":
      return t("history.kind.trash_restore");
    case "revert":
      return t("history.kind.revert");
  }
}

/**
 * clientText is where a changeset's writes came from: the web, the API,
 * the CLI, an MCP client by its name (mcp:<name>), or, as a filter, every
 * MCP client (mcp); a client this does not know, as it is.
 */
export function clientText(client: string, t: Translate): string {
  switch (client) {
    case "web":
      return t("history.client.web");
    case "api":
      return t("history.client.api");
    case "cli":
      return t("history.client.cli");
    case "mcp":
      return t("history.client.mcpAll");
  }
  return client.startsWith("mcp:") ? t("history.client.mcp", { name: client.slice("mcp:".length) }) : client;
}

/** byText is who made a changeset, and what it was: "Edit · Ada". */
export function byText(changeset: ChangesetBrief, t: Translate): string {
  return t("history.by", { kind: kindText(changeset.kind, t), name: changeset.created_by.display_name });
}

/**
 * changesetName is how a changeset is named where it leads (v0.1 design
 * 13.2, item 17): by its kind, who made it and when, to the minute, as
 * locale writes it; a twin, another named the same, by the end of its id
 * too (twinsIn).
 */
export function changesetName(changeset: ChangesetBrief, twin: boolean, t: Translate, locale: Locale): string {
  const kind = kindText(changeset.kind, t);
  const name = changeset.created_by.display_name;
  const time = formatDateTime(changeset.created_at, locale);
  return twin
    ? t("history.nameTwin", { kind, name, time, id: shortId(changeset.id) })
    : t("history.name", { kind, name, time });
}

/**
 * twinsIn are the ids of the changesets of list, those a page names, that
 * another of them has the name of: of one kind, by a name, two accounts
 * of one name too, in one minute as locale writes it (M8/P3 design 4.2,
 * as the trash's items, M8/P2B review A-M4; M8/P3B review A-M5). One
 * listed twice, a row and what another reverts, is one.
 */
export function twinsIn(list: readonly ChangesetBrief[], locale: Locale): Set<string> {
  const each = [...new Map(list.map((changeset) => [changeset.id, changeset])).values()];
  return twinsOf(
    each,
    (changeset) =>
      `${changeset.kind}\n${changeset.created_by.display_name}\n${formatDateTime(changeset.created_at, locale)}`
  );
}

/** countText writes a count the history counts up to 1,000, where 1,000 is as many or more. */
export function countText(count: number, t: Translate, locale: Locale): string {
  const written = new Intl.NumberFormat(locale).format(count);
  return count >= 1000 ? t("history.atLeast", { count: written }) : written;
}

/** countsText says how many nodes a changeset changed in the tree and how many pages' contents it wrote. */
export function countsText(changeset: Changeset, t: Translate, locale: Locale): string {
  return t("history.counts", {
    nodes: countText(changeset.node_count, t, locale),
    contents: countText(changeset.content_count, t, locale),
  });
}

/** changeText says what a change did to its node, as a log's row sums it up: "renamed “B” to “C”". */
function changeText(change: ChangesetChange, t: Translate): string {
  const name = change.after?.name ?? change.before?.name ?? change.node.name;
  switch (change.kind) {
    case "created":
      return t("history.change.created", { name });
    case "restored":
      return t("history.change.restored", { name });
    case "deleted":
      return t("history.change.deleted", { name });
    case "moved":
      return t("history.change.moved", { name });
    case "renamed":
      return t("history.change.renamed", { from: change.before?.name ?? "", to: name });
    case "reordered":
      return t("history.change.reordered", { name });
    case "content":
      return t("history.change.content", { name: change.node.name });
  }
}

/**
 * changesText sums up the changes a changeset lists with it, its first
 * three by node id: what each did, one after the other, as a sentence of
 * locale, and an ellipsis when it changed more: a node has one item and
 * one version at most in a changeset, so the items and versions not
 * listed are of nodes not listed (M8/P3B review B-1).
 */
export function changesText(changeset: Changeset, t: Translate, locale: Locale): string {
  const said = changeset.changes.map((change) => changeText(change, t)).join(t("history.separator"));
  const items = changeset.changes.filter((change) => change.kind !== "content").length;
  const versions = changeset.changes.filter((change) => change.content !== null).length;
  const more = changeset.node_count > items || changeset.content_count > versions ? "…" : "";
  return said === "" ? "" : `${said.charAt(0).toLocaleUpperCase(locale)}${said.slice(1)}${more}`;
}

/** reasonText says why the node of conflict keeps the revert from its changeset. */
export function reasonText(conflict: RevertConflict, t: Translate): string {
  const name = conflict.name;
  switch (conflict.reason) {
    case "changed":
      return t("history.conflict.changed", { name });
    case "expired":
      return t("history.conflict.expired", { name });
    case "parent_gone":
      return t("history.conflict.parent_gone", { name });
    case "children_added":
      return t("history.conflict.children_added", { name });
    case "title_taken":
      return t("history.conflict.title_taken", { name });
    case "cycle":
      return t("history.conflict.cycle", { name });
    case "too_deep":
      return t("history.conflict.too_deep", { name });
  }
}
