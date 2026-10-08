import { computed, makeAutoObservable, observableRef, runInAction } from "mobx";

import { oneAtATime } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type {
  BacklinkPage,
  LinkingService,
  LinkLanding,
  LinkTarget,
  PageProperties,
  TagCount,
} from "../services/linking.service";
import type { EditLock, NodeMove, PageService, PageView, TaskToggle, TreeNode } from "../services/page.service";
import { ancestorsOf, childrenOf, indexTree, subtreeOf, type TreeIndex } from "./page-tree";

/** How long a toggle of a task item holds its page's others back: more than the server takes to answer. */
export const toggleLimit = 60_000;

/**
 * PageTreeStore holds one notebook's page tree for one generation (M4/P5
 * design 3.4): the left column shows it, and a page's shell finds its page,
 * ancestors and children in it, so that the tree read again refreshes them
 * all. It also keeps which pages the left column shows open. The tree is
 * the pages': the notebook's attachments are nodes too, beside the pages
 * (M7/P2 design 3.10), but no level of the tree, and a read that changes
 * them alone does not render the tree again; only a new page's title
 * looks at them, as their names are taken.
 *
 * Its writes go out one at a time, in the order made, whichever page each
 * is of: one write moves its siblings, which the next may name (M4 design
 * 4). Each answer, a refusal too, has the tree read again; what a write
 * answers is not put in the tree here, since the client does not work out
 * the siblings' order. The pages this generation deleted leave the tree at
 * once, whether it could be read again or not.
 *
 * It reads for the notebook what its pages' links and tags lead to (M6/P6
 * design 13): the pages of a tag, which SWR keeps as it does the views,
 * and where a page made for a link would go, read anew each time.
 */
export class PageTreeStore {
  /** The nodes as read, pages and attachments, replaced whole by each read: they are not observed one by one. */
  nodes: TreeNode[] | undefined = undefined;
  /** The pages whose children the left column shows. */
  private readonly open = new Set<string>();
  /** How many writes have been answered: a read that overlaps one may have read the tree before it. */
  private changesAnswered = 0;
  /** How many reads have started, and which of them the tree is: an earlier one answered late does not replace it. */
  private readsStarted = 0;
  private readKept = 0;
  /** The pages this generation deleted, each with where its shell goes: the deleted subtree's parent (null: home). */
  private readonly removed = new Map<string, string | null>();
  private readonly inTurn = oneAtATime();
  /** The pages with a toggle of a task item out, the view read again included, each with when it started. */
  private readonly togglesOut = new Map<string, number>();

  constructor(
    private readonly service: Pick<
      PageService,
      | "listNodes"
      | "getPageView"
      | "createPage"
      | "renameNode"
      | "moveNode"
      | "deleteNode"
      | "lock"
      | "releaseLock"
      | "toggleTask"
    >,
    /** The notebook whose pages these are. */
    readonly notebookId: string,
    private readonly linking: Pick<
      LinkingService,
      "tagPages" | "linkLanding" | "linkTargets" | "tags" | "backlinks" | "properties"
    >
  ) {
    makeAutoObservable<
      this,
      "service" | "linking" | "changesAnswered" | "readsStarted" | "readKept" | "inTurn" | "togglesOut"
    >(this, {
      service: false,
      linking: false,
      notebookId: false,
      nodes: observableRef,
      pageNodes: computed({ equals: sameNodes }),
      changesAnswered: false,
      readsStarted: false,
      readKept: false,
      inTurn: false,
      togglesOut: false,
    });
  }

  /** pageNodes are the pages of the nodes read: the same while a read changes attachments alone. */
  get pageNodes(): TreeNode[] | undefined {
    return this.nodes?.filter((node) => node.kind === "page");
  }

  /** tree is the tree looked up, once read, without the pages this generation deleted. */
  get tree(): TreeIndex | undefined {
    const nodes = this.pageNodes;
    return nodes && indexTree(this.removed.size === 0 ? nodes : nodes.filter((node) => !this.removed.has(node.id)));
  }

  /**
   * siblingsOf are the nodes right under parent (null: the root), pages
   * and attachments, whose names a new page's title is not; none before
   * the tree is read.
   */
  siblingsOf(parent: string | null): readonly TreeNode[] {
    return this.nodes?.filter((node) => node.parent_id === parent && !this.removed.has(node.id)) ?? [];
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
   * load reads the tree; SWR calls it, each write's answer, and the events
   * of other tabs' writes. A write answered while the read was out is newer
   * than what it read: the tree kept is the one read after it. Reads that
   * overlap take effect in the order they started (M5/P3 design 3.9): one
   * answered after a later one is dropped. A tree read the same as before
   * is kept as it was, so that what shows it does not render again.
   */
  async load(): Promise<TreeNode[]> {
    const answeredBefore = this.changesAnswered;
    const read = ++this.readsStarted;
    const nodes = await this.service.listNodes(this.notebookId);
    if (this.changesAnswered !== answeredBefore) {
      return this.nodes ?? this.load();
    }
    if (read < this.readKept) {
      return this.nodes ?? nodes;
    }
    this.readKept = read;
    if (!sameNodes(this.nodes, nodes)) {
      runInAction(() => {
        this.nodes = nodes;
      });
    }
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
   * then go to the subtree's parent. The subtree is the one in the tree as
   * the deletion's turn comes: a page that a write before it created is in
   * it.
   */
  async remove(id: string): Promise<void> {
    await this.write(async () => {
      const tree = this.tree;
      const parent = this.byId(id)?.parent_id ?? null;
      try {
        await this.service.deleteNode(id);
      } catch (error) {
        if (!(error instanceof ApiError && error.code === "page.not_found")) {
          throw error;
        }
      }
      runInAction(() => {
        for (const page of tree === undefined ? [] : subtreeOf(tree, id)) {
          this.removed.set(page.id, parent);
        }
      });
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

  /**
   * wrote has the tree read again after a change of its nodes answered that
   * was not one of its writes, an upload of an attachment (M7/P4 design
   * 3.3), which goes out beside them: a read out meanwhile may have read
   * the tree before it.
   */
  async wrote(): Promise<void> {
    this.changesAnswered += 1;
    await this.load().catch(() => undefined);
  }

  /** view reads the page id's reading view, which the store does not keep: SWR does, by page. */
  view(id: string): Promise<PageView> {
    return this.service.getPageView(id);
  }

  /** tagPages reads the ids of the notebook's pages that have tag, or a tag under it (tag/…), by id. */
  tagPages(tag: string): Promise<string[]> {
    return this.linking.tagPages(this.notebookId, tag);
  }

  /** landing reads where a page made for target, a link's target on the page id, would go (M6/P6 design 2). */
  landing(id: string, target: string): Promise<LinkLanding> {
    return this.linking.linkLanding(id, target);
  }

  /** linkTargets reads what the notebook's links may lead to, for the editor's completion (M6/P7 design 3). */
  linkTargets(): Promise<LinkTarget[]> {
    return this.linking.linkTargets(this.notebookId);
  }

  /** tags reads the notebook's tags, each with how many pages have it. */
  tags(): Promise<TagCount[]> {
    return this.linking.tags(this.notebookId);
  }

  /** backlinks reads a page of the pages that link to the page id, after cursor (none for the first). */
  backlinks(id: string, cursor?: string): Promise<BacklinkPage> {
    return this.linking.backlinks(id, cursor);
  }

  /** properties reads the page id's properties, and where its property links lead. */
  properties(id: string): Promise<PageProperties> {
    return this.linking.properties(id);
  }

  /** toggleTask ticks or clears a task item of the page id, which changes its view: SWR reads it again. */
  async toggleTask(id: string, toggle: TaskToggle): Promise<void> {
    await this.service.toggleTask(id, toggle);
  }

  /**
   * oneToggle runs toggle, a toggle of a task item of the page id and the view read after it, unless one of the
   * page's runs (M5/P6 design 3.5): one is out per page at a time, across the views of the page and their HTML
   * read again, which a view's own state would not hold. One out for toggleLimit holds no longer: a request the
   * connection lost without an answer would hold the page's toggles until the next sign-in. It answers whether
   * toggle ran.
   */
  async oneToggle(id: string, toggle: () => Promise<void>): Promise<boolean> {
    const started = this.togglesOut.get(id);
    if (started !== undefined && Date.now() - started < toggleLimit) {
      return false;
    }
    const mine = Date.now();
    this.togglesOut.set(id, mine);
    try {
      await toggle();
      return true;
    } finally {
      if (this.togglesOut.get(id) === mine) {
        this.togglesOut.delete(id);
      }
    }
  }

  /** editLock reads who edits the page id, which SWR keeps by page, as it does the view (M5/P3 design 3.10). */
  editLock(id: string): Promise<EditLock> {
    return this.service.lock(id);
  }

  /** releaseEditLock ends the edit session that holds the page id's lock: its notebook's admins can. */
  releaseEditLock(id: string): Promise<void> {
    return this.service.releaseLock(id);
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

/** sameNodes tells whether two reads list the same nodes, in the same order, each as it was; none read is none. */
function sameNodes(kept: readonly TreeNode[] | undefined, read: readonly TreeNode[] | undefined): boolean {
  return (
    kept === read ||
    (kept !== undefined &&
      read !== undefined &&
      kept.length === read.length &&
      kept.every((node, i) => JSON.stringify(node) === JSON.stringify(read[i])))
  );
}
