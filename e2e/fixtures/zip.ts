import { crc32, inflateRawSync } from "node:zlib";

// The archives of the exports, read back (M7/P5 design 3.16): from the
// central directory, each entry's name, flags, method and bytes, which
// must have the size and the CRC-32 the directory gives. Only stored and
// deflated entries of an archive under 4 GiB, which the stories' are.

/** An entry of an archive: its name, whether it is named in UTF-8, how it is stored (0 stored, 8 deflated), its bytes. */
export interface ZipEntry {
  name: string;
  utf8: boolean;
  method: number;
  data: Buffer;
}

/** The entries of the zip archive in buf, in the central directory's order. */
export function unzip(buf: Buffer): ZipEntry[] {
  let end = buf.length - 22;
  while (end >= 0 && buf.readUInt32LE(end) !== 0x06054b50) {
    end--;
  }
  if (end < 0) {
    throw new Error("not a zip archive");
  }
  const entries: ZipEntry[] = [];
  let at = buf.readUInt32LE(end + 16);
  for (let i = buf.readUInt16LE(end + 10); i > 0; i--) {
    if (buf.readUInt32LE(at) !== 0x02014b50) {
      throw new Error("a broken central directory");
    }
    const flags = buf.readUInt16LE(at + 8);
    const method = buf.readUInt16LE(at + 10);
    const crc = buf.readUInt32LE(at + 16);
    const size = buf.readUInt32LE(at + 20);
    const unpacked = buf.readUInt32LE(at + 24);
    const nameLength = buf.readUInt16LE(at + 28);
    const local = buf.readUInt32LE(at + 42);
    const name = buf.toString("utf8", at + 46, at + 46 + nameLength);
    at += 46 + nameLength + buf.readUInt16LE(at + 30) + buf.readUInt16LE(at + 32);
    const start = local + 30 + buf.readUInt16LE(local + 26) + buf.readUInt16LE(local + 28);
    const raw = buf.subarray(start, start + size);
    if (method !== 0 && method !== 8) {
      throw new Error(`${name}: compression method ${method}`);
    }
    // Bit 11 tells a name in UTF-8; an ASCII name needs none.
    const utf8 = (flags & 0x800) !== 0 || /^[\x20-\x7e]*$/.test(name);
    const data = method === 0 ? Buffer.from(raw) : inflateRawSync(raw);
    if (data.length !== unpacked || crc32(data) !== crc) {
      throw new Error(`${name}: ${data.length.toString()} bytes, not the ${unpacked.toString()} its CRC-32 is of`);
    }
    entries.push({ name, utf8, method, data });
  }
  return entries;
}
