import { createHash } from "node:crypto";
import { crc32, deflateSync } from "node:zlib";

import { zipOf, type ZipFile } from "../fixtures/zip-write";

// The notebooks perf measures, each an import's vault (M12/P4 design 4.3, 4.7, 4.9): the page tree's three shapes,
// and the larger flat ones. An import sorts siblings by name (.nerve/meta.json is not written), and makes a page of
// each folder: "000 big" comes first at the root. Names and text come from a generator seeded per shape, so that a
// shape is the same archive on every run; its sha256 goes into the round's record.

/** The seed every shape's generator starts from, with its shape's name. */
export const seed = 20261116;

/**
 * The most nodes an import makes, its pages, attachments and the folders their paths imply (page.MaxJobItems, 45,000
 * since M12/P4/S6): a shape past it is imported in archives of at most this many, the same pages in the same tree.
 */
const importNodes = 45_000;

/** An archive of a shape, and the title of the page at the notebook's root it is imported under; none, the root. */
export interface Archive {
  readonly zip: Buffer;
  readonly under?: string;
}

/** A notebook to import. */
export interface Shape {
  /** Its notebook's name: the tree's navigation is "Pages of <name>". */
  readonly notebook: string;
  /** How many pages its root has once imported. */
  readonly roots: number;
  /** How many pages, and attachments, it has. */
  readonly pages: number;
  readonly attachments: number;
  /** The folder of 1,000 pages at its root that T3 opens; none in a flat shape. */
  readonly big?: string;
  /**
   * Its archives, imported one after another, each into its root or under the page it names: more than one past an
   * import's nodes (importNodes).
   */
  archives(): Archive[];
}

/** The shapes by name: NW_PERF_SHAPES names those a round imports. */
export const shapes = {
  flat: flat("Flat", 10_000, 5, [1]),
  deep: deep("Deep", false),
  attachments: deep("Attachments", true),
  flat3k: flat("Flat 3k", 3_000, 5, [1]),
  flat50k: flat("Flat 50k", 50_000, 6, [1]),
  flat100k: flat("Flat 100k", 100_000, 6, [1, 50_001]),
  folder50k: oneFolder("Folder 50k", 50_000),
} satisfies Record<string, Shape>;

export type ShapeName = keyof typeof shapes;

/** Whether name is a shape's. */
export function isShapeName(name: string): name is ShapeName {
  return Object.hasOwn(shapes, name);
}

/** count PNGs of exactly 1,024 bytes, each its own, from a generator seeded with label: uploads (design 4.7). */
export function pngsOf(label: string, count: number): Buffer[] {
  const random = generator(label);
  return Array.from({ length: count }, () => png(random));
}

/** The sha256 of an archive, in hexadecimal. */
export function sha256(archive: Buffer): string {
  return createHash("sha256").update(archive).digest("hex");
}

/**
 * flat is count pages at the root, "p00001 …" on, digits wide: a page each. Its pages are generated in runs starting
 * at the numbers of starts, each up to the next or to count, a generator seeded with its start; each run is cut in
 * even archives of at most importNodes pages (flat50k's one run of 50,000 in two since M12/P4/S6, flat100k's two in
 * four), so that its pages are the ones they were.
 */
function flat(notebook: string, count: number, digits: number, starts: number[]): Shape {
  return {
    notebook,
    roots: count,
    pages: count,
    attachments: 0,
    archives: () =>
      starts.flatMap((start, i) => {
        const end = (starts[i + 1] ?? count + 1) - 1;
        const random = generator(`${notebook} ${start}`);
        const files: ZipFile[] = [];
        for (let n = start; n <= end; n++) {
          files.push(page(`p${String(n).padStart(digits, "0")}`, random));
        }
        const parts = Math.ceil(files.length / importNodes);
        const size = Math.ceil(files.length / parts);
        return Array.from({ length: parts }, (_, k) => archiveOf(files.slice(k * size, (k + 1) * size)));
      }),
  };
}

/** The archive of files, imported under the page under, checked within an import's nodes. */
function archiveOf(files: ZipFile[], under?: string): Archive {
  const nodes = new Set<string>();
  for (const f of files) {
    const name = String(f.name);
    nodes.add(name);
    for (let at = name.indexOf("/"); at >= 0; at = name.indexOf("/", at + 1)) {
      nodes.add(name.slice(0, at + 1));
    }
  }
  if (nodes.size > importNodes) {
    throw new Error(`an archive of ${nodes.size} nodes, past an import's ${importNodes}`);
  }
  return under === undefined ? { zip: zipOf(files) } : { zip: zipOf(files), under };
}

/**
 * oneFolder is one folder page at the root, "Folder", holding count pages: an import that runs minutes and leaves the
 * tree's root one row (the jobs' polling, design 4.8). Past an import's nodes (folder50k since M12/P4/S6) it is two
 * archives of the same pages: the folder with its first count - importNodes pages, one at least, then the rest, at
 * most importNodes, the longest import there can be, under it. Two archives hold count from 45,000 to 89,999
 * (folder50k's 50,000: 5,000 and 45,000); past that archiveOf throws.
 */
function oneFolder(notebook: string, count: number): Shape {
  return {
    notebook,
    roots: 1,
    pages: count + 1,
    attachments: 0,
    archives: () => {
      const random = generator(notebook);
      const files: ZipFile[] = [];
      for (let n = 1; n <= count; n++) {
        files.push(page(`Folder/p${pad(n, 6)}`, random));
      }
      if (count + 1 <= importNodes) {
        return [archiveOf(files)];
      }
      const first = Math.max(1, count - importNodes);
      const rest = files.slice(first);
      for (const f of rest) {
        f.name = String(f.name).slice("Folder/".length);
      }
      return [archiveOf(files.slice(0, first)), archiveOf(rest, "Folder")];
    },
  };
}

/**
 * deep is ten roots, r01 to r10, each of ten folder pages, each of a hundred pages; and "000 big", a folder page of a
 * thousand pages, first at the root: 11,111 pages. With attachments, each of the hundred folder pages at the second
 * level also holds ten PNGs of 1 KiB: 1,000 attachments.
 */
function deep(notebook: string, attachments: boolean): Shape {
  return {
    notebook,
    roots: 11,
    pages: 11_111,
    attachments: attachments ? 1_000 : 0,
    big: "000 big",
    archives: () => {
      const random = generator(notebook);
      const files: ZipFile[] = [];
      for (let b = 1; b <= 1_000; b++) {
        files.push(page(`000 big/b${pad(b, 4)}`, random));
      }
      for (let r = 1; r <= 10; r++) {
        const root = `r${pad(r, 2)}`;
        for (let f = 1; f <= 10; f++) {
          const folder = `${root}/${root}-f${pad(f, 2)}`;
          for (let p = 1; p <= 100; p++) {
            files.push(page(`${folder}/${root}-f${pad(f, 2)}-p${pad(p, 3)}`, random));
          }
          if (attachments) {
            for (let a = 1; a <= 10; a++) {
              files.push({ name: `${folder}/a${pad(a, 2)}.png`, data: png(random) });
            }
          }
        }
      }
      return [archiveOf(files)];
    },
  };
}

function pad(n: number, digits: number): string {
  return String(n).padStart(digits, "0");
}

const words = (
  "alder basin cedar delta ember fable garnet harbor island juniper kestrel lantern meadow nectar orchard pebble " +
  "quarry raven saddle timber umber valley willow yarrow zephyr amber brook canyon dune estuary fjord grove " +
  "heath inlet jetty knoll lagoon marsh nook oasis prairie quay ridge summit tundra upland vale wharf"
).split(" ");

/** The page at path (its last part the title's start), its title ending in two words, its text a short paragraph. */
function page(path: string, random: () => number): ZipFile {
  const word = () => words[Math.floor(random() * words.length)] ?? "";
  const title = `${path.slice(path.lastIndexOf("/") + 1)} ${word()} ${word()}`;
  const text = Array.from({ length: 12 + Math.floor(random() * 13) }, word).join(" ");
  return { name: `${path.slice(0, path.lastIndexOf("/") + 1)}${title}.md`, data: `# ${title}\n\n${text}.\n` };
}

/**
 * A PNG of exactly 1,024 bytes: 238 × 4 grey pixels, random, stored without compression (the signature, IHDR and
 * IEND take 45 bytes, the IDAT chunk 12 and its zlib stream 11 besides the 956 bytes of rows).
 */
function png(random: () => number): Buffer {
  const image = pngOf(
    238,
    4,
    () => Math.floor(random() * 256),
    (rows) => deflateSync(rows, { level: 0 })
  );
  if (image.length !== 1_024) {
    throw new Error(`a PNG of ${image.length} bytes, not 1,024`);
  }
  return image;
}

/** A PNG of width × height grey pixels, each grey(x, y), its rows compressed by deflate (a zlib stream). */
export function pngOf(
  width: number,
  height: number,
  grey: (x: number, y: number) => number,
  deflate: (rows: Buffer) => Buffer
): Buffer {
  const rows = Buffer.alloc(height * (width + 1));
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      rows[y * (width + 1) + 1 + x] = grey(x, y);
    }
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header.set([8, 0, 0, 0, 0], 8); // 8 bits of grey; compression, filter and interlace methods 0
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", header),
    chunk("IDAT", deflate(rows)),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}

function chunk(type: string, data: Buffer): Buffer {
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length);
  const typed = Buffer.concat([Buffer.from(type, "latin1"), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(typed) >>> 0);
  return Buffer.concat([length, typed, crc]);
}

/** A generator of numbers in [0, 1), mulberry32 seeded from seed and label: the same label gives the same numbers. */
function generator(label: string): () => number {
  let state = createHash("sha256").update(`${seed} ${label}`).digest().readUInt32LE(0);
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let t = state;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4_294_967_296;
  };
}
