import { makeAutoObservable, observable, reaction, runInAction } from "mobx";

import { fixedName, freeName, isPageName } from "../lib/upload-name";
import { oneAtATimeById } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type { Asset, AssetService } from "../services/asset.service";
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
 * siblings, of the file's name fixed; how much of it has gone; why it
 * failed once it has, which shows until it is dismissed.
 */
export class Upload {
  name: string;
  sent = 0;
  total: number;
  /** failure is why the upload failed: an UploadRefusal, the server's refusal, or an error; undefined while it goes. */
  failure: unknown = undefined;
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
 * it. Each sends a fixed name that no sibling and no other upload has; one
 * taken meanwhile (409 page.title_taken: another tab's, or a name the
 * server compares otherwise) tries the next free one, three names at most.
 * Each answer, a refusal too, has the tree and its parent's list read
 * again; a sent upload leaves the uploads once its list has it. While any
 * goes, the page warns before it is left (unload). The generation's end
 * cancels them.
 *
 * A list's reads go one at a time, so that what each reads follows what
 * the one before it read: SWR's, those after a change, and More. A list
 * read again reads as many pages as it had. Renames, moves and deletions
 * are the tree's writes, in its queue (they change siblings' names, as
 * its other writes do); the lists they change are read again after them.
 */
export class AssetStore {
  /** The uploads going, and those that failed until dismissed, in the order they began. */
  uploads: Upload[] = [];
  /** The lists read, by parent (the root's under ""). */
  private readonly lists = observable.map<string, AssetList>({}, { deep: false });
  /** The parents whose lists have been asked for (the root's under ""): a change reads them again. */
  private readonly asked = new Set<string>();
  private uploadsStarted = 0;
  private readonly reading = oneAtATimeById();

  constructor(
    private readonly service: Pick<AssetService, "list" | "upload">,
    /** The notebook whose attachments these are. */
    readonly notebookId: string,
    private readonly pages: Pick<PageTreeStore, "siblingsOf" | "wrote" | "rename" | "move" | "remove">,
    /** Aborts as the generation ends: its uploads stop. */
    private readonly generation: AbortSignal,
    /** unload has the page warn before it is left, while on (v0.1 design 13.2, item 21). */
    unload: (on: boolean) => void
  ) {
    makeAutoObservable<this, "service" | "pages" | "generation" | "asked" | "uploadsStarted" | "reading">(this, {
      service: false,
      notebookId: false,
      pages: false,
      generation: false,
      asked: false,
      uploadsStarted: false,
      reading: false,
    });
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

  /** load reads the attachments under parent again, as many pages as it had (one the first time); SWR calls it. */
  load(parent: string | null): Promise<AssetList> {
    this.asked.add(parent ?? "");
    return this.reading(parent ?? "", async () => {
      const count = this.listOf(parent)?.pages ?? 1;
      const first = await this.service.list(this.notebookId, parent);
      const list: { assets: Asset[]; next: string | null; pages: number } = {
        assets: [...first.data],
        next: first.next_cursor,
        pages: 1,
      };
      while (list.next !== null && list.pages < count) {
        // oxlint-disable-next-line no-await-in-loop -- each page after the one before
        const page = await this.service.list(this.notebookId, parent, list.next);
        list.assets.push(...page.data);
        list.next = page.next_cursor;
        list.pages += 1;
      }
      runInAction(() => this.lists.set(parent ?? "", list));
      return list;
    });
  }

  /** more reads the next page of the attachments under parent, after those read, if there is one. */
  more(parent: string | null): Promise<void> {
    return this.reading(parent ?? "", async () => {
      const list = this.listOf(parent);
      if (list === undefined || list.next === null) {
        return;
      }
      const page = await this.service.list(this.notebookId, parent, list.next);
      runInAction(() =>
        this.lists.set(parent ?? "", {
          assets: [...list.assets, ...page.data],
          next: page.next_cursor,
          pages: list.pages + 1,
        })
      );
    });
  }

  /**
   * upload uploads each of files under parent (null: the root), side by
   * side, each by its name fixed (untitled for one of nothing) and free
   * among the siblings and the other uploads. One that is a page's file,
   * or larger than limits.maxBytes, is not sent: it shows why.
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

  /** send sends upload by the first name free beside the siblings, the other uploads and taken. */
  private async send(upload: Upload, taken: readonly string[]): Promise<void> {
    const name = freeName(upload.fixed, [
      ...this.pages.siblingsOf(upload.parent).map((node) => node.name),
      ...this.uploads.filter((other) => other !== upload && other.parent === upload.parent).map((other) => other.name),
      ...taken,
    ]);
    runInAction(() => {
      upload.name = name;
      upload.sent = 0;
    });
    try {
      await this.service.upload(
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
        // It may have reached the server before it stopped.
        this.dismiss(upload);
        await this.read([upload.parent]);
        return;
      }
      await this.read([upload.parent]);
      if (error instanceof ApiError && error.code === "page.title_taken" && taken.length + 1 < attempts) {
        return this.send(upload, [...taken, name]);
      }
      runInAction(() => {
        upload.failure = error;
      });
      return;
    }
    await this.read([upload.parent]);
    this.dismiss(upload);
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

/** isRefusal tells whether failure is an UploadRefusal: the upload was not sent. */
export function isRefusal(failure: unknown): failure is UploadRefusal {
  return typeof failure === "object" && failure !== null && "refused" in failure;
}
