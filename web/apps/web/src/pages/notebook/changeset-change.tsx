import { observer } from "mobx-react-lite";
import { useRef, type Ref } from "react";
import { Link } from "react-router";

import { useT, type Translate } from "../../i18n/i18n";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { shortId } from "../../lib/twins";
import type { Notebook } from "../../services/notebook.service";
import type { ChangesetChange } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import { openOf } from "../open-node";
import { useWorkspace } from "../workspace/workspace-layout";

/** placeText says where a node was: under a page, by its name now, at the notebook's top level, or under one purged. */
function placeText(state: NonNullable<ChangesetChange["before"]>, t: Translate): string {
  if (state.parent_id === null) {
    return t("history.detail.top");
  }
  return state.parent_name === null
    ? t("history.detail.purged")
    : t("history.detail.under", { name: state.parent_name });
}

/**
 * treeTexts say what a change did to its node in the tree: created it,
 * restored it from the trash, deleted it, moved it from one page to
 * another (renamed it too, where its names differ), renamed it, or put it
 * elsewhere among its siblings; nothing for a content's write alone.
 */
function treeTexts(change: ChangesetChange, t: Translate): string[] {
  const { before, after } = change;
  const renamed =
    before !== null && after !== null && before.name !== after.name
      ? [t("history.detail.renamed", { from: before.name, to: after.name })]
      : [];
  switch (change.kind) {
    case "created":
      return [t("history.detail.created")];
    case "restored":
      return [t("history.detail.restored")];
    case "deleted":
      return [t("history.detail.deleted")];
    case "moved":
      return before === null || after === null
        ? []
        : [t("history.detail.moved", { from: placeText(before, t), to: placeText(after, t) }), ...renamed];
    case "renamed":
      return renamed;
    case "reordered":
      return [t("history.detail.reordered")];
    case "content":
      return [];
  }
}

/**
 * ChangesetChange is a node a changeset changed, as its page lists them
 * (M8 design 4.7; M8/P3 design 4.3): the node by its name now, leading to
 * it alive (a page to itself, an attachment to where it is, as the
 * trash's Open), in the trash to its preview, which goes to its item's
 * root; a twin, another listed of its name, by the end of its id too
 * (twin); its history; then what the change did to it, and the version of
 * its content it wrote, leading to that version's page while the page is
 * alive: its comparison with the one before it is the change. Gone with
 * the focus, read away, it gives the focus away (left).
 */
export const ChangesetChangeRow = observer(function ChangesetChangeRow({
  notebook,
  change,
  twin,
  left,
  linkRef,
}: {
  notebook: Notebook;
  change: ChangesetChange;
  twin: boolean;
  left: () => void;
  linkRef?: Ref<HTMLAnchorElement>;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const row = useRef<HTMLLIElement>(null);
  useFocusLeaving(row, left);
  const home = `/${slug}/notebooks/${notebook.id}`;
  const { node, content } = change;
  const name = twin ? t("history.nodeTwin", { name: node.name, id: shortId(node.id) }) : node.name;
  // An attachment is where the tree has it now, the notebook's top level too (null), or where the change left it, the
  // tree not read yet, or read before it came (M8/P3B review A-M4).
  const now = pages.nodes?.find((each) => each.id === node.id);
  const parent = now === undefined ? (change.after?.parent_id ?? null) : now.parent_id;
  const open = openOf({ id: node.id, kind: node.kind, parent }, home);
  const texts = treeTexts(change, t);
  return (
    <li ref={row} className="space-y-1 p-4">
      <p className="flex flex-wrap items-center gap-x-3 gap-y-1">
        {node.alive ? (
          <Link
            ref={linkRef}
            to={open.to}
            state={open.state}
            aria-label={twin ? name : undefined}
            className="font-medium break-words underline underline-offset-4"
          >
            {node.name}
          </Link>
        ) : (
          <>
            <span className="font-medium break-words">{node.name}</span>
            <Link
              ref={linkRef}
              to={`${home}/trash/${node.id}`}
              aria-label={t("history.inTrashLabel", { name })}
              className="text-sm underline underline-offset-4"
            >
              {t("history.inTrash")}
            </Link>
          </>
        )}
        {twin && <span className="text-sm text-muted-foreground">{t("history.id", { id: shortId(node.id) })}</span>}
        <Link
          to={`${home}/history?page=${node.id}`}
          aria-label={t("history.nodeHistoryLabel", { name })}
          className="text-sm underline underline-offset-4"
        >
          {t("history.nodeHistory")}
        </Link>
      </p>
      {(texts.length > 0 || content !== null) && (
        <p className="flex flex-wrap gap-x-3 text-sm break-words text-muted-foreground">
          {texts.map((text) => (
            <span key={text}>{text}</span>
          ))}
          {content !== null &&
            (node.alive && node.kind === "page" ? (
              <Link
                to={`${home}/pages/${node.id}/versions/${content.revision.toString()}`}
                aria-label={t("history.detail.contentLabel", { name, revision: content.revision })}
                className="underline underline-offset-4"
              >
                {t("history.detail.content")}
              </Link>
            ) : (
              <span>{t("history.detail.content")}</span>
            ))}
        </p>
      )}
    </li>
  );
});
