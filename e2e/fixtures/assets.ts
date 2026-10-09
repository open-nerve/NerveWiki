import type { ApiClient, Asset } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";

// The attachments of the stories, through the API (M7/P2 design 3.13): an
// upload is multipart/form-data, which the client sends as a FormData; the
// content is read at the address the server signed, without a token.

/** A file a story uploads: its name, its bytes, and the type its part declares, which the server does not trust. */
export interface UploadFile {
  name: string;
  bytes: Uint8Array<ArrayBuffer>;
  type?: string;
}

/** credential's upload of file into the notebook notebookId, under parentId or at its root, as the API answers it. */
async function postAsset(api: ApiClient, credential: string, notebookId: string, file: UploadFile, parentId?: string) {
  const form = new FormData();
  if (parentId !== undefined) {
    form.append("parent_id", parentId);
  }
  form.append("file", new Blob([file.bytes], { type: file.type ?? "application/octet-stream" }), file.name);
  return api.POST("/api/v0/notebooks/{notebook_id}/assets", {
    params: { path: { notebook_id: notebookId } },
    body: { file: file.name },
    bodySerializer: () => form,
    headers: bearer(credential),
  });
}

/** Uploads file into the notebook notebookId with credential, under parentId or at its root, and returns it. */
export async function uploadAsset(
  api: ApiClient,
  credential: string,
  notebookId: string,
  file: UploadFile,
  parentId?: string
): Promise<Asset> {
  const { data, error, response } = await postAsset(api, credential, notebookId, file, parentId);
  expect(response.status, `upload ${file.name}: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`upload ${file.name} answered 201 without the attachment`);
  }
  return data;
}

/** credential's read of the attachment id, as the API answers it. */
export async function getAsset(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/assets/{node_id}", { params: { path: { node_id: id } }, headers: bearer(credential) });
}

/** The attachments under the page parentId of the notebook notebookId, or at its root, as credential lists them. */
export async function listAssets(
  api: ApiClient,
  credential: string,
  notebookId: string,
  parentId?: string
): Promise<Asset[]> {
  const { data, error, response } = await api.GET("/api/v0/notebooks/{notebook_id}/assets", {
    params: { path: { notebook_id: notebookId }, query: parentId === undefined ? {} : { parent_id: parentId } },
    headers: bearer(credential),
  });
  expect(response.status, `list the attachments: ${JSON.stringify(error)}`).toBe(200);
  return data?.data ?? [];
}

/** The answer to a GET of address, an attachment's content as the server signed it, at baseURL, without a token. */
export function download(baseURL: string, address: string, headers: Record<string, string> = {}): Promise<Response> {
  return fetch(new URL(address, baseURL), { headers });
}

/** The id of the file of the attachment a, its address's b. */
export function blobOf(a: Asset): string {
  const blob = new URL(a.content_url, "http://localhost").searchParams.get("b");
  if (blob === null) {
    throw new Error(`no blob in ${a.content_url}`);
  }
  return blob;
}

/** The bytes of a minimal PNG of 2 × 3 pixels, which the server reads the size of. */
export const pngBytes = Uint8Array.from(
  Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAIAAAADCAIAAAA2iEnWAAAAEElEQVR4nGP4z8AARAwoFABE0AX7pM/egAAAAABJRU5ErkJggg==",
    "base64"
  )
);

/** The bytes of text, UTF-8. */
export function utf8(text: string): Uint8Array<ArrayBuffer> {
  return new TextEncoder().encode(text);
}

/**
 * The bytes of an Ogg Opus file of seconds of silence, mono (RFC 7845):
 * its two header pages, then its frames of 20 ms, each the silent CELT
 * frame, 250 to a page. Playwright's Chromium plays Opus, not AAC.
 */
export function oggOpus(seconds: number): Uint8Array<ArrayBuffer> {
  const preSkip = 312;
  const head = Buffer.alloc(19);
  head.write("OpusHead", 0, "latin1");
  head.writeUInt8(1, 8);
  head.writeUInt8(1, 9);
  head.writeUInt16LE(preSkip, 10);
  head.writeUInt32LE(48_000, 12);
  const vendor = "nervewiki e2e";
  const tags = Buffer.alloc(8 + 4 + vendor.length + 4);
  tags.write("OpusTags", 0, "latin1");
  tags.writeUInt32LE(vendor.length, 8);
  tags.write(vendor, 12, "latin1");
  const frames = seconds * 50;
  const pages = [oggPage(0x02, 0, 0, [head]), oggPage(0, 0, 1, [tags])];
  for (let first = 0; first < frames; first += 250) {
    const count = Math.min(250, frames - first);
    const last = first + count === frames;
    const silence = Array.from({ length: count }, () => Buffer.from([0xf8, 0xff, 0xfe]));
    pages.push(oggPage(last ? 0x04 : 0, (first + count) * 960, pages.length, silence));
  }
  return Uint8Array.from(Buffer.concat(pages));
}

/** oggPage is an Ogg page of the stream's packets, each shorter than 255 bytes: its flags, granule and sequence. */
function oggPage(flags: number, granule: number, sequence: number, packets: Buffer[]): Buffer {
  const header = Buffer.alloc(27 + packets.length);
  header.write("OggS", 0, "latin1");
  header.writeUInt8(flags, 5);
  header.writeBigUInt64LE(BigInt(granule), 6);
  header.writeUInt32LE(1, 14);
  header.writeUInt32LE(sequence, 18);
  header.writeUInt8(packets.length, 26);
  packets.forEach((packet, at) => header.writeUInt8(packet.length, 27 + at));
  const page = Buffer.concat([header, ...packets]);
  page.writeUInt32LE(oggChecksum(page), 22);
  return page;
}

/** oggChecksum is an Ogg page's CRC-32: polynomial 0x04c11db7, unreflected, from 0, its own field 0. */
function oggChecksum(bytes: Uint8Array): number {
  let crc = 0;
  for (const byte of bytes) {
    crc ^= byte << 24;
    for (let bit = 0; bit < 8; bit++) {
      crc = crc & 0x80_00_00_00 ? (crc << 1) ^ 0x04_c1_1d_b7 : crc << 1;
    }
  }
  return crc >>> 0;
}
