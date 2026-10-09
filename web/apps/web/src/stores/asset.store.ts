import { makeAutoObservable, observable, reaction, runInAction } from "mobx";

import { oneAtATimeById } from "../lib/one-at-a-time";
import { fixedName, freeName, isPageName } from "../lib/upload-name";
import { ApiError } from "../services/api";
import type { Asset, AssetPage, AssetService } from "../services/asset.service";
import type { PageTreeStore } from "./page-tree.store";

/** How many names an upload tries before it gives up, as a page's creation does: the first free one and two more. */
const attempts = 3;

/**
 * AssetList is the attachments read under a page or the root: as many
 * pages of them as were asked for, the cursor of the next (null after the
 * last), and how many pages that is.
 */
export type AssetList = { assets: readonly Asset[]; next: string | null; pages: number };

/** Why an upload was not sent: it is a page's file, or larger than the instance takes. */
export type UploadRefusal = { refused: "page-file" } | { refused: "too-large"; max: number };

/** UploadLimits are what the page knows of what the server takes: the instance's largest attachment, in bytes. */
export type UploadLimits = { maxBytes: number | undefined };

/**
 * Upload is a file going up as an attachment (M7/P4 design 3.3): under its
 * parent (null: the root), by the name it is sent with, free among its
 * siblings, of the file's name fixed; how much of it has gone, and whether
 * the server has answered it (it is no longer to be cancelled then); why
 * it failed once it has, which shows until it is dismissed; and, once
 * answered, the attachment it made.
 */
export class Upload {
  name: string;
  sent = 0;
  total: number;
  answered = false;
  /** failure is why the upload failed: an UploadRefusal, the server's refusal, or an error; undefined while it goes. */
  failure: unknown = undefined;
  /** uploaded is the attachment it made, once answered. */
  uploaded: Asset | undefined = undefined;
  private readonly controller = new AbortController();

  constructor(
    readonly key: number,
    readonly parent: string | null,
    /** The file's name fixed, which each name tried is free from. */
    readonly fixed: string,
    readonly file: Blob
  ) {
    this.name = fixed;
    this.total = file.size;
    makeAutoObservable<this, "controller">(this, {
      key: false,
      parent: false,
      fixed: false,
      file: false,
      controller: false,
    });
  }

  /** signal aborts as the upload is cancelled. */
  get signal(): AbortSignal {
    return this.controller.signal;
  }

  /** cancel stops the upload: its request rejects, and it leaves the list. */
  cancel(): void {
    this.controller.abort();
  }
}

/**
 * AssetStore is a notebook's attachments for one generation (M7/P4 design
 * 3.3): the lists of them read, under a page or the root, and the uploads.
 *
 * Uploads go side by side, not in the tree's queue of writes (v0.1 design
 * 13.2, item 1): a large file would hold every write of the tree behind
 * it. Each sends a fixed name that no sibling and no other upload going
 * has; one taken meanwhile (409 page.title_taken: another tab's, or a name
 * the server compares otherwise) tries the next free one, three names at
 * most. Each answer, a refusal too, has the tree and its parent's list
 * read again; a sent upload leaves the uploads once they are and its list
 * has its attachment, the pages after those read read as far as it is.
 * While any goes, the page warns before it is left (unload). The
 * generation's end cancels them.
 *
 * A list's reads go one at a time, so that what each reads follows what
 * the one before it read: SWR's, those after a change, and More; a read
 * asked for while another waits its turn is that one. A list read again
 * reads as many pages as it had, each attachment once. Renames, moves and
 * deletions are the tree's writes, in its queue (they change siblings'
 * names, as its other writes do); the lists they change are read again
 * after them.
 */
export class AssetStore {
  /** The uploads going, and those that failed until dismissed, in the order they began. */
  uploads: Upload[] = [];
  /** The lists read, by parent (the root's under ""). */
  private readonly lists = observable.map<string, AssetList>({}, { deep: false });
  /** The parents whose lists have been asked for (the root's under ""): a change reads them again. */
  private readonly asked = new Set<string>();
  /** The reads of a list waiting their turn, by parent: one asked for meanwhile is the same. */
  private readonly waiting = new Map<string, Promise<AssetList>>();
  private uploadsStarted = 0;
  private readonly reading = oneAtATimeById();

  constructor(
    private readonly service: Pick<AssetService, "list" | "upload" | "get">,
    /** The notebook whose attachments these are. */
    readonly notebookId: string,
    private readonly pages: Pick<PageTreeStore, "siblingsOf" | "wrote" | "rename" | "move" | "remove">,
    /** Aborts as the generation ends: its uploads stop. */
    private readonly generation: AbortSignal,
    /** unload has the page warn before it is left, while on (v0.1 design 13.2, item 21). */
    unload: (on: boolean) => void
  ) {
    makeAutoObservable<this, "service" | "pages" | "generation" | "asked" | "waiting" | "uploadsStarted" | "reading">(
      this,
      {
        service: false,
        notebookId: false,
        pages: false,
        generation: false,
        asked: false,
        waiting: false,
        uploadsStarted: false,
        reading: false,
      }
    );
    reaction(() => this.going.length > 0, unload);
    generation.addEventListener("abort", () => {
      for (const upload of this.uploads) {
        upload.cancel();
      }
    });
  }

  /** going are the uploads still going. */
  get going(): Upload[] {
    return this.uploads.filter((upload) => upload.failure === undefined);
  }

  /** listOf is the attachments read under parent (null: the root); undefined before the first read answers. */
  listOf(parent: string | null): AssetList | undefined {
    return this.lists.get(parent ?? "");
  }

  /**
   * load reads the attachments under parent again, as many pages as it had
   * (one the first time); SWR calls it. One asked for while another waits
   * its turn is that one.
   */
  load(parent: string | null): Promise<AssetList> {
    const key = parent ?? "";
    this.asked.add(key);
    const waiting = this.waiting.get(key);
    if (waiting !== undefined) {
      return waiting;
    }
    const read = this.reading(key, async () => {
      this.waiting.delete(key);
      const count = this.listOf(parent)?.pages ?? 1;
      let list = listed(undefined, await this.service.list(this.notebookId, parent));
      while (list.next !== null && list.pages < count) {
        // oxlint-disable-next-line no-await-in-loop -- each page after the one before
        list = listed(list, await this.service.list(this.notebookId, parent, list.next));
      }
      const whole = list;
      runInAction(() => this.lists.set(key, whole));
      return whole;
    });
    this.waiting.set(key, read);
    return read;
  }

  /**
   * more reads the next page of the attachments under parent, after those
   * read, if there is one; it answers the attachments it added.
   */
  more(parent: string | null): Promise<readonly Asset[]> {
    return this.reading(parent ?? "", async () => {
      const list = this.listOf(parent);
      if (list === undefined || list.next === null) {
        return [];
      }
      const next = listed(list, await this.service.list(this.notebookId, parent, list.next));
      runInAction(() => this.lists.set(parent ?? "", next));
      const before = new Set(list.assets.map((asset) => asset.id));
      return next.assets.filter((asset) => !before.has(asset.id));
    });
  }

  /**
   * upload uploads each of files under parent (null: the root), side by
   * side, each by its name fixed (untitled for one of nothing) and free
   * among the siblings and the other uploads going. One that is a page's
   * file, or larger than limits.maxBytes, is not sent: it shows why.
   */
  upload(parent: string | null, files: readonly File[], untitled: string, limits: UploadLimits): Upload[] {
    return files.map((file) => {
      const upload = new Upload(++this.uploadsStarted, parent, fixedName(file.name, untitled), file);
      this.uploads.push(upload);
      if (isPageName(upload.name)) {
        upload.failure = { refused: "page-file" } satisfies UploadRefusal;
      } else if (limits.maxBytes !== undefined && file.size > limits.maxBytes) {
        upload.failure = { refused: "too-large", max: limits.maxBytes } satisfies UploadRefusal;
      } else {
        if (this.generation.aborted) {
          upload.cancel();
        }
        void this.send(upload, []);
      }
      return upload;
    });
  }

  /** dismiss takes upload off the uploads: a failed one, once read. */
  dismiss(upload: Upload): void {
    this.uploads = this.uploads.filter((other) => other !== upload);
  }

  /** address is the address of the attachment id's content, signed anew (M7/P4 design 4.6). */
  async address(id: string): Promise<string> {
    return (await this.service.get(id)).content_url;
  }

  /** rename renames the attachment id under parent, one of the tree's writes, and reads its list again. */
  async rename(id: string, parent: string | null, name: string): Promise<void> {
    await this.changing([parent], () => this.pages.rename(id, name));
  }

  /** move moves the attachment id from parent to the page to (null: the root), last, and reads both lists again. */
  async move(id: string, parent: string | null, to: string | null): Promise<void> {
    await this.changing([parent, to], () => this.pages.move(id, { parent_id: to }));
  }

  /** remove deletes the attachment id under parent, and reads its list again; one gone already is gone as well. */
  async remove(id: string, parent: string | null): Promise<void> {
    await this.changing([parent], () => this.pages.remove(id));
  }

  /** send sends upload by the first name free beside the siblings, the other uploads going and taken. */
  private async send(upload: Upload, taken: readonly string[]): Promise<void> {
    const name = freeName(upload.fixed, [
      ...this.pages.siblingsOf(upload.parent).map((node) => node.name),
      ...this.going.filter((other) => other !== upload && other.parent === upload.parent).map((other) => other.name),
      ...taken,
    ]);
    runInAction(() => {
      upload.name = name;
      upload.sent = 0;
    });
    let asset: Asset;
    try {
      asset = await this.service.upload(
        this.notebookId,
        { parent: upload.parent, name, file: upload.file },
        {
          progress: (sent, total) =>
            runInAction(() => {
              upload.sent = sent;
              upload.total = total;
            }),
          signal: upload.signal,
        }
      );
    } catch (error) {
      if (upload.signal.aborted) {
        this.dismiss(upload);
      }
      // Cancelled, it may have reached the server before it stopped: the tree and the list are read all the same.
      await this.read([upload.parent]);
      if (upload.signal.aborted) {
        this.dismiss(upload);
        return;
      }
      if (error instanceof ApiError && error.code === "page.title_taken" && taken.length + 1 < attempts) {
        return this.send(upload, [...taken, name]);
      }
      runInAction(() => {
        upload.failure = error;
      });
      return;
    }
    runInAction(() => {
      upload.answered = true;
      upload.uploaded = asset;
    });
    await this.read([upload.parent]);
    await this.reach(asset);
    this.dismiss(upload);
  }

  /**
   * reach reads the pages after those read under asset's parent, one at a
   * time, until one has asset, uploaded: the upload shows where it went,
   * and the list read again has it, as many pages as it has read. A list
   * read whole without it has lost it meanwhile (deleted, moved); a read
   * that fails stops.
   */
  private async reach(asset: Asset): Promise<void> {
    for (;;) {
      const list = this.listOf(asset.parent_id);
      if (list === undefined || list.next === null || list.assets.some((each) => each.id === asset.id)) {
        return;
      }
      try {
        // oxlint-disable-next-line no-await-in-loop -- each page after the one before
        await this.more(asset.parent_id);
      } catch {
        return;
      }
    }
  }

  /** changing runs change, one of the tree's writes, then reads the lists under parents again. */
  private async changing(parents: readonly (string | null)[], change: () => Promise<void>): Promise<void> {
    try {
      await change();
    } finally {
      await this.read(parents, false);
    }
  }

  /**
   * read reads the lists under parents again, those asked for, and the tree
   * as told; a read that fails leaves them as they were.
   */
  private async read(parents: readonly (string | null)[], tree = true): Promise<void> {
    await Promise.all([
      tree ? this.pages.wrote() : undefined,
      ...[...new Set(parents)]
        .filter((parent) => this.asked.has(parent ?? ""))
        .map((parent) => this.load(parent).catch(() => undefined)),
    ]);
  }
}

/**
 * listed is list with page read after it, each attachment once: one a
 * change moved past the cursor between the reads is where the later read
 * has it.
 */
function listed(list: AssetList | undefined, page: AssetPage): AssetList {
  const read = new Set(page.data.map((asset) => asset.id));
  return {
    assets: [...(list?.assets.filter((asset) => !read.has(asset.id)) ?? []), ...page.data],
    next: page.next_cursor,
    pages: (list?.pages ?? 0) + 1,
  };
}

/** isRefusal tells whether failure is an UploadRefusal: the upload was not sent. */
export function isRefusal(failure: unknown): failure is UploadRefusal {
  return typeof failure === "object" && failure !== null && "refused" in failure;
}
