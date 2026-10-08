/** AssetKind is what an attachment is, by its type, as its list shows it: what the server shows, or a file. */
export type AssetKind = "image" | "audio" | "video" | "pdf" | "file";

/**
 * assetKind is the kind of an attachment served as mime (M7/P4 design
 * 3.5): the server shows an image, an audio, a video and a PDF, and serves
 * any other file as application/octet-stream, downloaded.
 */
export function assetKind(mime: string): AssetKind {
  const type = mime.split(";", 1)[0]?.trim().toLowerCase() ?? "";
  if (type === "application/pdf") {
    return "pdf";
  }
  const top = type.split("/", 1)[0];
  return top === "image" || top === "audio" || top === "video" ? top : "file";
}

/** opensInline tells whether an attachment served as mime opens in the browser, in a tab of its own, or downloads. */
export function opensInline(mime: string): boolean {
  return assetKind(mime) !== "file";
}

/** embedOf is the wikilink that embeds the attachment link leads to: ![[link]]. */
export function embedOf(link: string): string {
  return `![[${link}]]`;
}
