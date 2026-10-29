import type { PageTreeStore } from "../stores/page-tree.store";
import { titleKey } from "../lib/title-key";
import { writesPages } from "./effective-role";

/** Current body capability, including callbacks kept across a tree or notebook refresh. */
export function canEditPageBody(pages: PageTreeStore, id: string): boolean {
  const notebook = pages.notebook;
  return (
    notebook !== undefined && writesPages(notebook.role) && pages.nodeById(id)?.kind === "page" && !pages.isSource(id)
  );
}

/** Only the notebook's root AGENTS is reserved; a nested namesake is an ordinary node. */
export function isAgentsPage(pages: PageTreeStore, id: string): boolean {
  const node = pages.nodeById(id);
  return (
    pages.notebook?.mode === "llm_wiki" &&
    node?.kind === "page" &&
    node.parent_id === null &&
    titleKey(node.name) === titleKey("AGENTS")
  );
}

/** Rename/move/reorder share the current role and the all-node source identity. */
export function canManageNode(pages: PageTreeStore, id: string): boolean {
  const notebook = pages.notebook;
  if (
    notebook === undefined ||
    !writesPages(notebook.role) ||
    pages.nodeById(id) === undefined ||
    isAgentsPage(pages, id)
  )
    return false;
  return (
    notebook.role === "admin" ||
    !(pages.isSource(id) || (notebook.mode === "llm_wiki" && notebook.source_root_id === id))
  );
}

/** The source root and its ancestors cannot be deleted, even by an admin. */
export function canDeleteNode(pages: PageTreeStore, id: string): boolean {
  if (!canManageNode(pages, id)) return false;
  const notebook = pages.notebook;
  const root = notebook?.mode === "llm_wiki" ? notebook.source_root_id : null;
  return root == null || (id !== root && !(pages.nodeAncestors(root)?.some((node) => node.id === id) ?? false));
}
