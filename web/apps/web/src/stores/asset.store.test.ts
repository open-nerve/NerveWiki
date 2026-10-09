import { describe, expect, test, vi } from "vitest";

import type { Asset, AssetUpload, UploadOptions } from "../services/asset.service";
import { ApiError } from "../services/api";
import type { TreeNode } from "../services/page.service";
import { assetJSON, assetNode, guide } from "../test/page-server";
import { AssetStore, isRefusal } from "./asset.store";

/** An upload sent: what went, how it is told its progress, and how the test answers it. */
type Sent = AssetUpload & {
  options: UploadOptions;
  resolve: (asset: Asset) => void;
  reject: (error: unknown) => void;
};

/**
 * setUp is an AssetStore of a notebook whose attachments under each parent
 * are in lists, two a page, siblings beside them in the tree; uploads wait
 * for the test to answer them (sent), and an abort rejects them as the
 * transfer does.
 */
function setUp(siblings: TreeNode[] = []) {
  const lists = new Map<string, Asset[]>();
  const sent: Sent[] = [];
  const asked: string[] = [];
  const service = {
    list: vi.fn(async (_notebook: string, parent: string | null, cursor?: string) => {
      asked.push(`${parent ?? "root"}${cursor === undefined ? "" : ` from ${cursor}`}`);
      const all = lists.get(parent ?? "") ?? [];
      const at = Number(cursor ?? "0");
      return { data: all.slice(at, at + 2), next_cursor: at + 2 < all.length ? String(at + 2) : null };
    }),
    upload: vi.fn(
      (_notebook: string, upload: AssetUpload, options: UploadOptions = {}) =>
        new Promise<Asset>((resolve, reject) => {
          sent.push({ ...upload, options, resolve, reject });
          if (options.signal?.aborted) {
            reject(new DOMException("stopped", "AbortError"));
          }
          options.signal?.addEventListener("abort", () => reject(new DOMException("stopped", "AbortError")));
        })
    ),
  };
  const pages = {
    siblingsOf: vi.fn((_parent: string | null) => siblings),
    wrote: vi.fn(async () => undefined),
    rename: vi.fn(async (_id: string, _name: string) => undefined),
    move: vi.fn(async (_id: string, _move: { parent_id: string | null }) => undefined),
    remove: vi.fn(async (_id: string) => undefined),
  };
  const generation = new AbortController();
  const unload: boolean[] = [];
  const store = new AssetStore(service, "n1", pages, generation.signal, (on) => unload.push(on));
  const add = (parent: string | null, ...names: string[]) => {
    const list = lists.get(parent ?? "") ?? [];
    list.push(...names.map((name, i) => assetJSON({ ...assetNode(60 + list.length + i, name), parent_id: parent })));
    lists.set(parent ?? "", list);
  };
  return { store, service, pages, sent, asked, generation, unload, add };
}

const file = (name: string, size = 3) => new File(["x".repeat(size)], name);
const refusal = (status: number, code: string) => new ApiError(status, { status, code, title: code });
const limits = { maxBytes: 100 };

/** settle lets the promises out settle. */
async function settle(): Promise<void> {
  for (let i = 0; i < 10; i++) {
    // oxlint-disable-next-line no-await-in-loop -- a turn at a time
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

describe("AssetStore's lists", () => {
  test("load reads the first page; read again, as many pages as it had", async () => {
    const { store, add, asked } = setUp();
    add(guide.id, "a.png", "b.png", "c.png", "d.png", "e.png");

    await store.load(guide.id);
    expect(store.listOf(guide.id)?.assets.map((asset) => asset.name)).toEqual(["a.png", "b.png"]);
    await store.more(guide.id);
    expect(store.listOf(guide.id)).toMatchObject({ next: "4", pages: 2 });
    asked.length = 0;
    await store.load(guide.id);

    expect(asked).toEqual([guide.name === "Guide" ? guide.id : "", `${guide.id} from 2`]);
    expect(store.listOf(guide.id)?.assets.map((asset) => asset.name)).toEqual(["a.png", "b.png", "c.png", "d.png"]);
  });

  test("more after the last page reads nothing; the root's list is its own", async () => {
    const { store, add, asked } = setUp();
    add(null, "a.png");

    await store.load(null);
    await store.more(null);

    expect(asked).toEqual(["root"]);
    expect(store.listOf(guide.id)).toBeUndefined();
  });

  test("a list's reads go one at a time: More read as a read is out follows it", async () => {
    const { store, add, service } = setUp();
    add(guide.id, "a.png", "b.png", "c.png");
    await store.load(guide.id);
    let answer: (() => void) | undefined;
    const first = service.list.getMockImplementation();
    service.list.mockImplementationOnce(async (...args) => {
      await new Promise<void>((resolve) => (answer = resolve));
      return first!(...args);
    });

    const reading = store.load(guide.id);
    const more = store.more(guide.id);
    await settle();
    expect(service.list).toHaveBeenCalledTimes(2);
    answer?.();
    await Promise.all([reading, more]);

    expect(store.listOf(guide.id)?.assets.map((asset) => asset.name)).toEqual(["a.png", "b.png", "c.png"]);
  });
});

describe("AssetStore's uploads", () => {
  test("go side by side, each by a name free among the siblings and the other uploads", async () => {
    const { store, sent } = setUp([{ ...assetNode(80, "a.png"), parent_id: guide.id }]);

    store.upload(guide.id, [file("a.png"), file("a.png"), file("b.png")], "Untitled", limits);
    await settle();

    expect(sent.map((each) => each.name)).toEqual(["a 2.png", "a 3.png", "b.png"]);
    expect(store.uploads.map((upload) => upload.name)).toEqual(["a 2.png", "a 3.png", "b.png"]);
  });

  test("a name is fixed first: one of nothing is untitled", async () => {
    const { store, sent } = setUp();

    store.upload(null, [file("a:b.png"), file("...")], "Untitled", limits);
    await settle();

    expect(sent.map((each) => [each.name, each.parent])).toEqual([
      ["a_b.png", null],
      ["Untitled", null],
    ]);
  });

  test("tells its progress", async () => {
    const { store, sent } = setUp();

    const [upload] = store.upload(null, [file("a.png", 10)], "Untitled", limits);
    await settle();
    sent[0]?.options.progress?.(4, 10);

    expect(upload).toMatchObject({ sent: 4, total: 10 });
  });

  test("answered, it has the tree and its list read again, and leaves the uploads once the list has it", async () => {
    const { store, sent, add, pages, asked } = setUp();
    await store.load(guide.id);
    asked.length = 0;

    store.upload(guide.id, [file("a.png")], "Untitled", limits);
    await settle();
    add(guide.id, "a.png");
    sent[0]?.resolve(assetJSON(assetNode(90, "a.png")));
    await settle();

    expect(pages.wrote).toHaveBeenCalledTimes(1);
    expect(asked).toEqual([guide.id]);
    expect(store.listOf(guide.id)?.assets.map((asset) => asset.name)).toEqual(["a.png"]);
    expect(store.uploads).toEqual([]);
  });

  test("a sent upload stays among the uploads until its list read again has it", async () => {
    const { store, sent, service, add } = setUp();
    await store.load(guide.id);
    let answer: (() => void) | undefined;
    const first = service.list.getMockImplementation();
    service.list.mockImplementationOnce(async (...args) => {
      await new Promise<void>((resolve) => (answer = resolve));
      return first!(...args);
    });

    store.upload(guide.id, [file("a.png")], "Untitled", limits);
    await settle();
    add(guide.id, "a.png");
    sent[0]?.resolve(assetJSON(assetNode(90, "a.png")));
    await settle();
    expect(store.uploads.map((upload) => upload.name)).toEqual(["a.png"]);
    expect(store.listOf(guide.id)?.assets).toEqual([]);
    answer?.();
    await settle();

    expect(store.uploads).toEqual([]);
    expect(store.listOf(guide.id)?.assets.map((asset) => asset.name)).toEqual(["a.png"]);
  });

  test("a list asked for and not read yet is read again too, after its first read", async () => {
    const { store, sent, service, add } = setUp();
    let answer: (() => void) | undefined;
    const first = service.list.getMockImplementation();
    service.list.mockImplementationOnce(async (...args) => {
      const page = await first!(...args);
      await new Promise<void>((resolve) => (answer = resolve));
      return page;
    });
    const reading = store.load(guide.id);

    store.upload(guide.id, [file("a.png")], "Untitled", limits);
    await settle();
    add(guide.id, "a.png");
    sent[0]?.resolve(assetJSON(assetNode(90, "a.png")));
    await settle();
    expect(service.list).toHaveBeenCalledTimes(1);
    answer?.();
    await reading;
    await settle();

    expect(service.list).toHaveBeenCalledTimes(2);
    expect(store.listOf(guide.id)?.assets.map((asset) => asset.name)).toEqual(["a.png"]);
    expect(store.uploads).toEqual([]);
  });

  test("a name taken meanwhile (409 page.title_taken) tries the next free one, three names at most", async () => {
    const { store, sent, pages } = setUp();

    const [upload] = store.upload(null, [file("a.png")], "Untitled", limits);
    await settle();
    sent[0]?.reject(refusal(409, "page.title_taken"));
    await settle();
    sent[1]?.reject(refusal(409, "page.title_taken"));
    await settle();
    sent[2]?.reject(refusal(409, "page.title_taken"));
    await settle();

    expect(sent.map((each) => each.name)).toEqual(["a.png", "a 2.png", "a 3.png"]);
    expect(pages.wrote).toHaveBeenCalledTimes(3);
    expect(upload?.failure).toMatchObject({ code: "page.title_taken" });
    expect(store.uploads).toEqual([upload]);
  });

  test("another refusal shows until dismissed", async () => {
    const { store, sent } = setUp();

    const [upload] = store.upload(null, [file("a.png")], "Untitled", limits);
    await settle();
    sent[0]?.reject(refusal(507, "storage_full"));
    await settle();
    expect(sent).toHaveLength(1);
    expect(upload?.failure).toMatchObject({ code: "storage_full" });

    store.dismiss(upload!);

    expect(store.uploads).toEqual([]);
  });

  test("a page's file, or one larger than the instance takes, is not sent: it says why", async () => {
    const { store, sent } = setUp();

    const uploads = store.upload(
      null,
      [file("notes.MD"), file("big.png", 101), file("ok.png", 100)],
      "Untitled",
      limits
    );
    await settle();

    expect(uploads.map((upload) => upload.failure)).toEqual([
      { refused: "page-file" },
      { refused: "too-large", max: 100 },
      undefined,
    ]);
    expect(uploads.map((upload) => isRefusal(upload.failure))).toEqual([true, true, false]);
    expect(sent.map((each) => each.name)).toEqual(["ok.png"]);
  });

  test("cancelled, it leaves the uploads, and the tree is read again: it may have arrived", async () => {
    const { store, sent, pages } = setUp();

    const [upload] = store.upload(null, [file("a.png")], "Untitled", limits);
    await settle();
    upload?.cancel();
    await settle();

    expect(sent[0]?.options.signal?.aborted).toBe(true);
    expect(store.uploads).toEqual([]);
    expect(pages.wrote).toHaveBeenCalledTimes(1);
  });

  test("the generation's end cancels them all; one begun after is cancelled at once", async () => {
    const { store, sent, generation } = setUp();

    store.upload(null, [file("a.png"), file("b.png")], "Untitled", limits);
    await settle();
    generation.abort();
    await settle();
    expect(sent.map((each) => each.options.signal?.aborted)).toEqual([true, true]);
    expect(store.uploads).toEqual([]);

    store.upload(null, [file("c.png")], "Untitled", limits);
    await settle();
    expect(store.uploads).toEqual([]);
  });

  test("the page warns before it is left while any goes, and no longer once none does", async () => {
    const { store, sent, unload } = setUp();

    store.upload(null, [file("a.png"), file("b.png")], "Untitled", limits);
    await settle();
    expect(unload).toEqual([true]);
    sent[0]?.resolve(assetJSON(assetNode(90, "a.png")));
    await settle();
    expect(unload).toEqual([true]);
    sent[1]?.reject(refusal(507, "storage_full"));
    await settle();

    expect(unload).toEqual([true, false]);
  });
});

describe("AssetStore's uploads, as they fail and go on", () => {
  test("one failed, or refused, holds no name: the next sends it", async () => {
    const { store, sent } = setUp();

    store.upload(null, [file("a.png", 101), file("b.png")], "Untitled", limits);
    await settle();
    sent[0]?.reject(refusal(507, "storage_full"));
    await settle();
    store.upload(null, [file("a.png"), file("b.png")], "Untitled", limits);
    await settle();

    expect(sent.map((each) => each.name)).toEqual(["b.png", "a.png", "b.png"]);
  });

  test("names are taken under one parent: an upload under another page takes its own", async () => {
    const { store, sent, pages } = setUp();
    pages.siblingsOf.mockImplementation((parent) =>
      parent === guide.id ? [{ ...assetNode(80, "a.png"), parent_id: guide.id }] : []
    );

    store.upload(guide.id, [file("a.png")], "Untitled", limits);
    store.upload(null, [file("a.png")], "Untitled", limits);
    await settle();

    expect(sent.map((each) => [each.parent, each.name])).toEqual([
      [guide.id, "a 2.png"],
      [null, "a.png"],
    ]);
  });

  test("cancelled as the tree is read after its failure, it leaves the uploads, no failure shown", async () => {
    const { store, sent, pages } = setUp();
    let answer: (() => void) | undefined;
    pages.wrote.mockImplementationOnce(() => new Promise<undefined>((resolve) => (answer = () => resolve(undefined))));

    const [upload] = store.upload(null, [file("a.png")], "Untitled", limits);
    await settle();
    sent[0]?.reject(refusal(507, "storage_full"));
    await settle();
    upload?.cancel();
    answer?.();
    await settle();

    expect(store.uploads).toEqual([]);
    expect(upload?.failure).toBeUndefined();
  });

  test("answered, it is no longer to be cancelled; its attachment is in its list though on a page not read", async () => {
    const { store, sent, add } = setUp();
    add(guide.id, "a.png", "b.png", "c.png");
    await store.load(guide.id);

    const [upload] = store.upload(guide.id, [file("z.png")], "Untitled", limits);
    await settle();
    const made = assetJSON({ ...assetNode(91, "z.png"), parent_id: guide.id });
    add(guide.id, "z.png");
    sent[0]?.resolve(made);
    await Promise.resolve();
    expect(upload?.answered).toBe(true);
    await settle();

    expect(store.listOf(guide.id)?.assets.map((asset) => asset.name)).toEqual(["a.png", "b.png", "z.png"]);
    expect(upload?.uploaded).toEqual(made);
  });
});

describe("AssetStore's lists, as they change", () => {
  test("pages read after a change list each attachment once, where the later read has it", async () => {
    const { store, add, service } = setUp();
    add(guide.id, "a.png", "b.png", "c.png");
    await store.load(guide.id);
    // a.png renamed past the cursor by another tab: the next page has it again.
    const first = service.list.getMockImplementation();
    service.list.mockImplementationOnce(async (...args) => {
      const page = await first!(...args);
      const renamed = store.listOf(guide.id)?.assets[0];
      return { ...page, data: [...page.data, ...(renamed === undefined ? [] : [{ ...renamed, name: "z.png" }])] };
    });

    const added = await store.more(guide.id);

    expect(store.listOf(guide.id)?.assets.map((asset) => asset.name)).toEqual(["b.png", "c.png", "z.png"]);
    expect(added.map((asset) => asset.name)).toEqual(["c.png"]);
  });

  test("a read asked for while another waits its turn is that one", async () => {
    const { store, add, service } = setUp();
    add(guide.id, "a.png");
    let answer: (() => void) | undefined;
    const first = service.list.getMockImplementation();
    service.list.mockImplementationOnce(async (...args) => {
      await new Promise<void>((resolve) => (answer = resolve));
      return first!(...args);
    });

    const out = store.load(guide.id);
    await settle();
    const waiting = store.load(guide.id);
    const again = store.load(guide.id);
    answer?.();
    await Promise.all([out, waiting, again]);

    expect(waiting).toBe(again);
    expect(service.list).toHaveBeenCalledTimes(2);
  });
});

describe("AssetStore's changes", () => {
  test("a rename, a move and a deletion are the tree's writes, and the lists they change are read again", async () => {
    const { store, pages, asked } = setUp();
    await store.load(guide.id);
    await store.load(null);
    asked.length = 0;

    await store.rename("a1", guide.id, "b.png");
    expect(pages.rename).toHaveBeenCalledWith("a1", "b.png");
    expect(asked).toEqual([guide.id]);

    asked.length = 0;
    await store.move("a1", guide.id, null);
    expect(pages.move).toHaveBeenCalledWith("a1", { parent_id: null });
    expect(asked.toSorted()).toEqual([guide.id, "root"].toSorted());

    asked.length = 0;
    await store.remove("a1", null);
    expect(pages.remove).toHaveBeenCalledWith("a1");
    expect(asked).toEqual(["root"]);
    expect(pages.wrote).not.toHaveBeenCalled();
  });

  test("a refused change reads its lists again all the same, and throws", async () => {
    const { store, pages, asked } = setUp();
    await store.load(guide.id);
    asked.length = 0;
    pages.rename.mockRejectedValueOnce(refusal(409, "page.title_taken"));

    await expect(store.rename("a1", guide.id, "b.png")).rejects.toMatchObject({ code: "page.title_taken" });

    expect(asked).toEqual([guide.id]);
  });
});
