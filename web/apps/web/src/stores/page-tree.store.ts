import { makeAutoObservable, observableRef, runInAction } from "mobx";

import { oneAtATime } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type { NodeMove, PageService, PageView, TreeNode } from "../services/page.service";
import { ancestorsOf, childrenOf, indexTree, subtreeOf, type TreeIndex } from "./page-tree";

/**
 * PageTreeStore holds one notebook's page tree for one generation (M4/P5
 * design 3.4): the left column shows it, and a page's shell finds its page,
 * ancestors and children in it, so that the tree read again refreshes them
 * all. It also keeps which pages the left column shows open.
 *
 * Its writes go out one at a time, in the order made, whichever page each
 * is of: one write moves its siblings, which the next may name (M4 design
 * 4). Each answer, a refusal too, has the tree read again; what a write
 * answers is not put in the tree here, since the client does not work out
 * the siblings' order.
 */
export class PageTreeStore {
  /** The tree as read, replaced whole by each read: its nodes are not observed one by one. */
  nodes: TreeNode[] | undefined = undefined;
  /** The pages whose children the left column shows. */
  private readonly open = new Set<string>();
  /** How many writes have been answered: a read that overlaps one may have read the tree before it. */
  private changesAnswered = 0;
  /** The pages this generation deleted, each with where its shell goes: the deleted subtree's parent (null: home). */
  private readonly removed = new Map<string, string | null>();
  private readonly inTurn = oneAtATime();

  constructor(
    private readonly service: Pick<
      PageService,
      "listNodes" | "getPageView" | "createPage" | "renameNode" | "moveNode" | "deleteNode"
    >,
    /** The notebook whose pages these are. */
    readonly notebookId: string
  ) {
    makeAutoObservable<this, "service" | "changesAnswered" | "removed" | "inTurn">(this, {
      service: false,
      notebookId: false,
      nodes: observableRef,
      changesAnswered: false,
      removed: false,
      inTurn: false,
    });
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

  /**
   * removedTo is where the shell of the page id goes once this generation
   * deleted it, with the subtree it was in: that subtree's parent (null:
   * the notebook's home); undefined for a page it did not delete.
   */
  removedTo(id: string): string | null | undefined {
    return this.removed.get(id);
  }

  /**
   * load reads the tree; SWR calls it, and each write's answer. A write
   * answered while the read was out is newer than what it read: the tree
   * kept is the one read after it.
   */
  async load(): Promise<TreeNode[]> {
    const answeredBefore = this.changesAnswered;
    const nodes = await this.service.listNodes(this.notebookId);
    if (this.changesAnswered !== answeredBefore) {
      return this.nodes ?? this.load();
    }
    runInAction(() => {
      this.nodes = nodes;
    });
    return nodes;
  }

  /** create creates a page titled title under parent (null: the root), last among its siblings, and answers its id. */
  async create(parent: string | null, title: string): Promise<string> {
    return (await this.write(() => this.service.createPage(this.notebookId, parent, title))).id;
  }

  async rename(id: string, name: string): Promise<void> {
    await this.write(() => this.service.renameNode(id, name));
  }

  async move(id: string, move: NodeMove): Promise<void> {
    await this.write(() => this.service.moveNode(id, move));
  }

  /**
   * remove deletes the page id with the pages under it; one deleted
   * already, or no longer seen, is gone as well. The shells of those pages
   * then go to the subtree's parent.
   */
  async remove(id: string): Promise<void> {
    const tree = this.tree;
    const parent = this.byId(id)?.parent_id ?? null;
    await this.write(async () => {
      try {
        await this.service.deleteNode(id);
      } catch (error) {
        if (!(error instanceof ApiError && error.code === "page.not_found")) {
          throw error;
        }
      }
      for (const page of tree === undefined ? [] : subtreeOf(tree, id)) {
        this.removed.set(page.id, parent);
      }
    });
  }

  /**
   * write sends send in turn and, once answered either way, reads the tree
   * again before it settles: a page created is in the tree once create
   * answers. A read that fails leaves the tree as it was, until SWR reads
   * it again.
   */
  private write<T>(send: () => Promise<T>): Promise<T> {
    return this.inTurn(async () => {
      try {
        return await send();
      } finally {
        this.changesAnswered += 1;
        await this.load().catch(() => undefined);
      }
    });
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
