import { useNavigate } from "react-router";

import { useMounted } from "../app/mounted";
import type { TreeNode } from "../services/page.service";
import type { PageTreeStore } from "../stores/page-tree.store";
import { openOf } from "./open-node";
import { useWorkspace } from "./workspace/workspace-layout";

export type NodeTarget = { node: TreeNode; path: string };

/** Canonical notebook paths include all ancestors and the final page's .md suffix. */
export function nodeTarget(pages: PageTreeStore, id: string): NodeTarget | undefined {
  const node = pages.nodeById(id);
  const ancestors = pages.nodeAncestors(id);
  if (
    node === undefined ||
    ancestors === undefined ||
    [node, ...ancestors].some((each) => each.notebook_id !== pages.notebookId)
  )
    return undefined;
  return {
    node,
    path: [...ancestors.map((each) => each.name), node.name].join("/") + (node.kind === "page" ? ".md" : ""),
  };
}

/** A node is looked up again at navigation time, including an attachment's current parent. */
export function useNodeNavigate(pages: PageTreeStore): (id: string) => void {
  const navigate = useNavigate();
  const { slug } = useWorkspace();
  const mounted = useMounted();
  return (id) => {
    const target = nodeTarget(pages, id);
    if (!mounted() || pages.notebook === undefined || target === undefined) return;
    const open = openOf(
      { id, kind: target.node.kind, parent: target.node.parent_id },
      `/${slug}/notebooks/${pages.notebookId}`
    );
    void navigate(open.to, { state: open.state });
  };
}
