import { makeAutoObservable, observableRef, runInAction } from "mobx";

import type { PageService, PageView, TreeNode } from "../services/page.service";
import { ancestorsOf, childrenOf, indexTree, type TreeIndex } from "./page-tree";

/**
 * PageTreeStore holds one notebook's page tree for one generation (M4/P5
 * design 3.4): the left column shows it, and a page's shell finds its page,
 * ancestors and children in it, so that the tree read again refreshes them
 * all. It also keeps which pages the left column shows open.
 */
export class PageTreeStore {
  /** The tree as read, replaced whole by each read: its nodes are not observed one by one. */
  nodes: TreeNode[] | undefined = undefined;
  /** The pages whose children the left column shows. */
  private readonly open = new Set<string>();

  constructor(
    private readonly service: Pick<PageService, "listNodes" | "getPageView">,
    /** The notebook whose pages these are. */
    readonly notebookId: string
  ) {
    makeAutoObservable<this, "service">(this, { service: false, notebookId: false, nodes: observableRef });
  }

  /** tree is the tree looked up, once read. */
  get tree(): TreeIndex | undefined {
    return this.nodes && indexTree(this.nodes);
  }

  /** byId is the page id in the tree, if the tree has it. */
  byId(id: string): TreeNode | undefined {
    return this.tree?.byId.get(id);
  }

  /** childrenOf are the pages under parent (null: the root), in their order; none before the tree is read. */
  childrenOf(parent: string | null): readonly TreeNode[] {
    return this.tree === undefined ? [] : childrenOf(this.tree, parent);
  }

  /** ancestorsOf are the pages above id, from the root down. */
  ancestorsOf(id: string): TreeNode[] {
    return this.tree === undefined ? [] : ancestorsOf(this.tree, id);
  }

  /** load reads the tree; SWR calls it. */
  async load(): Promise<TreeNode[]> {
    const nodes = await this.service.listNodes(this.notebookId);
    runInAction(() => {
      this.nodes = nodes;
    });
    return nodes;
  }

  /** view reads the page id's reading view, which the store does not keep: SWR does, by page. */
  view(id: string): Promise<PageView> {
    return this.service.getPageView(id);
  }

  isOpen(id: string): boolean {
    return this.open.has(id);
  }

  toggle(id: string): void {
    if (this.open.has(id)) {
      this.open.delete(id);
    } else {
      this.open.add(id);
    }
  }

  /** openTo opens the ancestors of the page id, so that the left column shows it. */
  openTo(id: string): void {
    for (const ancestor of this.ancestorsOf(id)) {
      this.open.add(ancestor.id);
    }
  }
}
