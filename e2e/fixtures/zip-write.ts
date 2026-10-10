import { crc32, deflateRawSync } from "node:zlib";

// The archives the import stories send (M7/P6 design 3.18), written with
// node's zlib, no dependency: an entry may be what no zip tool writes, as
// a malicious archive holds it.

/** An entry of an archive a story writes. */
export interface ZipFile {
  /** Its name; bytes for one that is not UTF-8. A name ending in "/" is a folder's, holding nothing. */
  name: string | Buffer;
  data?: Uint8Array | string;
  /**
   * How its data is stored: 0 as it is, 8 deflated (a file's default); another method's number stores its data as it
   * is, under that number.
   */
  method?: number;
  /** A symbolic link: Unix's file type in its external attributes. */
  symlink?: boolean;
  /** Flagged encrypted (bit 0), its data as it is. */
  encrypted?: boolean;
  /** Its CRC-32 wrong by one. */
  badCrc?: boolean;
  /** A comment in its central directory record: at most 65,535 bytes. */
  comment?: Buffer;
}

/** What an archive's end record tells, instead of the truth. */
export interface ZipEnd {
  /** The entries it counts. */
  entries?: number;
}

/** The archive of files, in their order, its end record as end says. Every size is below 4 GiB: no zip64. */
export function zipOf(files: ZipFile[], end: ZipEnd = {}): Buffer {
  const locals: Buffer[] = [];
  const records: Buffer[] = [];
  let offset = 0;
  for (const f of files) {
    const name = typeof f.name === "string" ? Buffer.from(f.name, "utf8") : f.name;
    const data = typeof f.data === "string" ? Buffer.from(f.data, "utf8") : Buffer.from(f.data ?? []);
    const folder = name.at(-1) === 0x2f;
    const method = f.method ?? (folder ? 0 : 8);
    const stored = method === 8 ? deflateRawSync(data) : data;
    const crc = (crc32(data) + (f.badCrc ? 1 : 0)) >>> 0;
    // Bit 11: the name is UTF-8, unless it is given as bytes.
    const flags = (typeof f.name === "string" ? 0x800 : 0) | (f.encrypted ? 0x1 : 0);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(flags, 6);
    local.writeUInt16LE(method, 8);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(stored.length, 18);
    local.writeUInt32LE(data.length, 22);
    local.writeUInt16LE(name.length, 26);
    locals.push(local, name, stored);

    const comment = f.comment ?? Buffer.alloc(0);
    const record = Buffer.alloc(46);
    record.writeUInt32LE(0x02014b50, 0);
    // Made by Unix (3), so that the external attributes hold its mode.
    record.writeUInt16LE((3 << 8) | 20, 4);
    record.writeUInt16LE(20, 6);
    record.writeUInt16LE(flags, 8);
    record.writeUInt16LE(method, 10);
    record.writeUInt32LE(crc, 16);
    record.writeUInt32LE(stored.length, 20);
    record.writeUInt32LE(data.length, 24);
    record.writeUInt16LE(name.length, 28);
    record.writeUInt16LE(comment.length, 32);
    const mode = f.symlink ? 0o120777 : folder ? 0o40755 : 0o100644;
    record.writeUInt32LE((mode << 16) >>> 0, 38);
    record.writeUInt32LE(offset, 42);
    records.push(record, name, comment);
    offset += local.length + name.length + stored.length;
  }
  const directory = Buffer.concat(records);
  const count = end.entries ?? files.length;
  const record = Buffer.alloc(22);
  record.writeUInt32LE(0x06054b50, 0);
  record.writeUInt16LE(count & 0xffff, 8);
  record.writeUInt16LE(count & 0xffff, 10);
  record.writeUInt32LE(directory.length, 12);
  record.writeUInt32LE(offset, 16);
  return Buffer.concat([...locals, directory, record]);
}
