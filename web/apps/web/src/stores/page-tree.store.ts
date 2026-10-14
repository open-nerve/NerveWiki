import { computed, makeAutoObservable, observableRef, runInAction } from "mobx";

import { oneAtATime, oneAtATimeById } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type {
  BacklinkPage,
  LinkingService,
  LinkLanding,
  LinkTarget,
  PageProperties,
  TagCount,
} from "../services/linking.service";
import type {
  ChangesetChangePage,
  ChangesetDetail,
  ChangesetPage,
  ChangesetReverted,
  EditLock,
  LogFilter,
  NodeMove,
  NodeRestored,
  PageContent,
  PageService,
  PageVersionContent,
  PageVersionPage,
  PageView,
  TaskToggle,
  TrashItem,
  TrashItemPage,
  TreeNode,
  VersionRestored,
} from "../services/page.service";
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
 * once, whether it could be read again or not, until a read begun after
 * the deletion's answer lists one again: restored from the trash, here,
 * in another tab or by someone else (M8 design 4.13).
 *
 * It reads for the notebook what its pages' links and tags lead to (M6/P6
 * design 13): the pages of a tag, which SWR keeps as it does the views,
 * and where a page made for a link would go, read anew each time.
 *
 * It reads a page's versions, which SWR keeps as it does the views, and
 * restores one (M8 design 4.3, 4.4): a page's restores go out one at a
 * time, in the order made, beside the tree's writes, which they do not
 * change. It reads the notebook's trash and an item of it, which SWR
 * keeps, and restores an item (M8 design 4.5, 4.6): a node's restores go
 * out one at a time too, each then in turn with the tree's writes, which
 * it is one of. It reads the notebook's history, which SWR keeps, and
 * reverts a changeset of it (M8/P3 design 4.6): a changeset's reverts go
 * out one at a time too, each in turn with the tree's writes, which it is
 * one of; the pages it deletes are this generation's deleted as well.
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
  /** The read started last, which a read that a change's answer overlapped answers instead of itself. */
  private latest: { read: number; nodes: Promise<TreeNode[]> } | undefined = undefined;
  /**
   * The reads after uploads' answers (wrote): whether one is out, and the
   * next, which those answered as it is out wait for together.
   */
  private readonly rereading: { out: boolean; next: { done: Promise<void>; settle: () => void } | undefined } = {
    out: false,
    next: undefined,
  };
  /**
   * The pages this generation deleted, each with where its shell goes, the
   * deleted subtree's parent (null: home), and how many reads had started
   * as the deletion answered: a read begun after it that lists the page
   * alive takes it out (M8/P2 design 4.4).
   */
  private readonly removed = new Map<string, { to: string | null; after: number }>();
  private readonly inTurn = oneAtATime();
  /** The pages with a toggle of a task item out, the view read again included, each with when it started. */
  private readonly togglesOut = new Map<string, number>();
  private readonly restores = oneAtATimeById();

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
      | "getPageContent"
      | "versions"
      | "version"
      | "versionView"
      | "restoreVersion"
      | "trash"
      | "trashItem"
      | "restoreNode"
      | "changesets"
      | "changeset"
      | "changes"
      | "revertChangeset"
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
      | "service"
      | "linking"
      | "changesAnswered"
      | "readsStarted"
      | "readKept"
      | "latest"
      | "rereading"
      | "inTurn"
      | "togglesOut"
      | "restores"
    >(this, {
      service: false,
      linking: false,
      notebookId: false,
      nodes: observableRef,
      pageNodes: computed({ equals: sameNodes }),
      changesAnswered: false,
      readsStarted: false,
      readKept: false,
      latest: false,
      rereading: false,
      inTurn: false,
      togglesOut: false,
      restores: false,
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
    return this.removed.get(id)?.to;
  }

  /**
   * load reads the tree; SWR calls it, each write's answer, and the events
   * of other tabs' writes. A write answered while the read was out is newer
   * than what it read: the tree kept is the one read after it, which that
   * read answers too (the answer is followed by a read at once), so that a
   * write that awaits its read settles with the tree as it left it, an
   * upload's answer overlapping it or not. Reads that overlap take effect
   * in the order they started (M5/P3 design 3.9): one answered after a
   * later one is dropped. A tree read the same as before is kept as it was,
   * so that what shows it does not render again. A page this generation
   * deleted that a read kept lists alive, a read begun after the
   * deletion's answer, is no longer deleted: it was restored since (M8/P2
   * design 4.4), and is in the tree again, a read the same as before too.
   */
  load(): Promise<TreeNode[]> {
    const read = ++this.readsStarted;
    const nodes = this.read(read);
    this.latest = { read, nodes };
    return nodes;
  }

  private async read(read: number): Promise<TreeNode[]> {
    const answeredBefore = this.changesAnswered;
    const nodes = await this.service.listNodes(this.notebookId);
    if (this.changesAnswered !== answeredBefore) {
      const latest = this.latest;
      // That read failing, the tree is read again.
      return latest !== undefined && latest.read > read ? latest.nodes.catch(() => this.load()) : this.load();
    }
    if (read < this.readKept) {
      return this.nodes ?? nodes;
    }
    this.readKept = read;
    const back = [...this.removed].filter(([id, { after }]) => read > after && nodes.some((node) => node.id === id));
    const same = sameNodes(this.nodes, nodes);
    if (!same || back.length > 0) {
      runInAction(() => {
        if (!same) {
          this.nodes = nodes;
        }
        for (const [id] of back) {
          this.removed.delete(id);
        }
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
        // The read after the answer, and those begun later, may take the pages out again.
        const after = this.readsStarted;
        for (const page of tree === undefined ? [] : subtreeOf(tree, id)) {
          this.removed.set(page.id, { to: parent, after });
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
   * the tree before it. Uploads answered as one such read is out have one
   * more read after it, not one each; each settles once the first tree
   * read begun after its answer is.
   */
  wrote(): Promise<void> {
    const rereading = this.rereading;
    if (rereading.next !== undefined) {
      return rereading.next.done;
    }
    let settle!: () => void;
    const done = new Promise<void>((resolve) => {
      settle = resolve;
    });
    rereading.next = { done, settle };
    if (!rereading.out) {
      void this.reread();
    }
    return done;
  }

  /** reread reads the tree for the uploads answered, one read at a time, until none waits. */
  private async reread(): Promise<void> {
    const rereading = this.rereading;
    rereading.out = true;
    for (let next = rereading.next; next !== undefined; next = rereading.next) {
      rereading.next = undefined;
      this.changesAnswered += 1;
      // oxlint-disable-next-line no-await-in-loop -- one read at a time
      await this.load().catch(() => undefined);
      next.settle();
    }
    rereading.out = false;
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

  /** content reads the page id's content as it is now, with its revision: what a version is compared with and restored on. */
  content(id: string): Promise<PageContent> {
    return this.service.getPageContent(id);
  }

  /** versions reads a page of the page id's versions, newer first, after cursor (none for the first). */
  versions(id: string, cursor?: string): Promise<PageVersionPage> {
    return this.service.versions(id, cursor);
  }

  /** version reads the version of the page id that holds revision, with its content and its own revision. */
  version(id: string, revision: number): Promise<PageVersionContent> {
    return this.service.version(id, revision);
  }

  /** versionView reads the reading view of the version of the page id that holds revision. */
  versionView(id: string, revision: number): Promise<PageView> {
    return this.service.versionView(id, revision);
  }

  /**
   * restoreVersion writes the content of the version of the page id that
   * holds revision as the page's, on base, the revision the page was read
   * at, after the page's restores made before it have settled: a dialog
   * mounted again may make one while the last is out. It answers what the
   * restore did; SWR reads what it changed.
   */
  restoreVersion(id: string, revision: number, base: number): Promise<VersionRestored> {
    return this.restores(id, () => this.service.restoreVersion(id, revision, base));
  }

  /** trash reads a page of the notebook's trash items, newer first, after cursor (none for the first). */
  trash(cursor?: string): Promise<TrashItemPage> {
    return this.service.trash(this.notebookId, cursor);
  }

  /** trashItem reads the trash item whose root is the node id, which SWR keeps, by node, as it does the views. */
  trashItem(id: string): Promise<TrashItem> {
    return this.service.trashItem(id);
  }

  /**
   * restore brings back the trash item whose root is the node id (M8/P2
   * design 4.4): after the node's restores made before it have settled,
   * a button pressed twice or the list's and the preview's, then in turn
   * with the tree's writes, the tree read again once it is answered either
   * way; the read lists its pages, which takes them out of those this
   * generation deleted. That read failed, a page this generation deleted
   * restored would stay deleted here, its Open leading to where it was:
   * the tree is read once more (M8/P2B review C, nit 1). A refusal records
   * nothing. It answers where the item went, and under which name.
   */
  async restore(id: string): Promise<NodeRestored> {
    const restored = await this.restores(id, () => this.write(() => this.service.restoreNode(id)));
    if (this.removed.has(id)) {
      await this.load().catch(() => undefined);
    }
    return restored;
  }

  /** changesets reads a page of the notebook's history that filter keeps, newer first, after cursor (none for the first). */
  changesets(filter: LogFilter, cursor?: string): Promise<ChangesetPage> {
    return this.service.changesets(this.notebookId, filter, cursor);
  }

  /** changeset reads the changeset id of the history, which SWR keeps, by changeset, as it does the views. */
  changeset(id: string): Promise<ChangesetDetail> {
    return this.service.changeset(id);
  }

  /** changes reads a page of the changeset id's changes, by node id, after cursor (none for the first). */
  changes(id: string, cursor?: string): Promise<ChangesetChangePage> {
    return this.service.changes(id, cursor);
  }

  /**
   * revert puts every node the changeset id changed back as it was before
   * it, with message, or without one (M8/P3 design 4.6): after the
   * changeset's reverts made before it have settled, a button pressed
   * twice or a dialog opened again, then in turn with the tree's writes,
   * the tree read again once it is answered either way. The answer does not
   * say which pages it deleted: those the tree had before it that a read
   * begun after the answer has not are this generation's deleted, as
   * remove records them, each shell then going to its nearest ancestor
   * still there, until a read begun after the answer lists one alive (one
   * someone else deleted meanwhile among them, gone all the same). It
   * brings pages back too, which that read lists: that read failed, a page
   * this generation deleted would stay deleted here, so the tree is read
   * once more (as restore does). A refusal records nothing. It answers what
   * the revert did.
   */
  async revert(id: string, message?: string): Promise<ChangesetReverted> {
    let before: TreeIndex | undefined;
    let after = Infinity;
    const reverted = await this.restores(id, () =>
      this.write(async () => {
        before = this.tree;
        const answer = await this.service.revertChangeset(id, message);
        after = this.readsStarted;
        return answer;
      })
    );
    // Whether a read begun after the answer was kept: the write's, or the one more.
    const read = () => this.readKept > after;
    if (!read() && this.removed.size > 0) {
      await this.load().catch(() => undefined);
    }
    const tree = before;
    const nodes = this.nodes;
    if (reverted.changed && tree !== undefined && nodes !== undefined && read()) {
      const alive = new Set(nodes.map((node) => node.id));
      runInAction(() => {
        for (const page of tree.byId.values()) {
          if (!alive.has(page.id) && !this.removed.has(page.id)) {
            const to = ancestorsOf(tree, page.id).findLast((ancestor) => alive.has(ancestor.id))?.id ?? null;
            this.removed.set(page.id, { to, after });
          }
        }
      });
    }
    return reverted;
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
