import { Prec, StateEffect, StateField } from "@codemirror/state";
import { EditorView } from "@codemirror/view";

import { embedOf } from "../../lib/asset-kind";
import { carriesFiles, filesDropped } from "../../lib/file-transfer";
import { isPageName } from "../../lib/upload-name";
import type { Asset } from "../../services/asset.service";
import type { Build, EditorContext, EditorControls } from "../registry";

/** A place where the embeds of files pasted or dropped together go, by its number, as the content changes. */
type Place = { id: number; at: number };

const placeAdded = StateEffect.define<Place>();
const placeDone = StateEffect.define<number>();

/**
 * places are where the embeds of uploads going are to be inserted, by
 * number: where they were pasted or dropped, mapped through each change
 * since, before what is typed there (the embed was pasted first); after
 * each embed inserted at one, the next of its files goes after it.
 */
const places = StateField.define<ReadonlyMap<number, number>>({
  create: () => new Map(),
  update(value, transaction) {
    let next: Map<number, number> | undefined;
    const changed = () => (next ??= new Map(value));
    if (!transaction.changes.empty) {
      for (const [id, at] of value) {
        changed().set(id, transaction.changes.mapPos(at, -1));
      }
    }
    for (const effect of transaction.effects) {
      if (effect.is(placeAdded)) {
        changed().set(effect.value.id, effect.value.at);
      } else if (effect.is(placeDone)) {
        changed().delete(effect.value);
      }
    }
    return next ?? value;
  },
});

/** The extensions of images by their types, as Obsidian names a pasted image's. */
const imageExtensions: Readonly<Record<string, string>> = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/gif": "gif",
  "image/webp": "webp",
  "image/bmp": "bmp",
  "image/svg+xml": "svg",
  "image/avif": "avif",
};

/** two writes n in two digits. */
function two(n: number): string {
  return n.toString().padStart(2, "0");
}

/** The name a browser gives an image from the clipboard that had none: image.png. */
const clipboardImage = /^image\.[a-z0-9]+$/i;

/**
 * pastedName is the name file, pasted, is uploaded by: an image the
 * clipboard held without a name of its own is "Pasted image" and the
 * local time to the second, as Obsidian names one, whatever the
 * reader's language (it is stored); any other file keeps its own.
 */
export function pastedName(file: File, now: Date): string {
  if (!file.type.startsWith("image/") || !(file.name === "" || clipboardImage.test(file.name))) {
    return file.name;
  }
  const stamp =
    now.getFullYear().toString() +
    two(now.getMonth() + 1) +
    two(now.getDate()) +
    two(now.getHours()) +
    two(now.getMinutes()) +
    two(now.getSeconds());
  const extension = imageExtensions[file.type] ?? file.type.slice("image/".length).replace(/[^a-z0-9]/gi, "");
  return `Pasted image ${stamp}.${extension}`;
}

/**
 * assetUpload uploads the files pasted into the editor or dropped on it
 * as attachments of the page, and inserts each one's embed, ![[link]],
 * where it went (M7/P4 design 5.2): it takes them before CodeMirror,
 * which would read a file in as text (an event taken, CodeMirror
 * prevents its default).
 *
 * - A paste of files only, no text (cells copied from a spreadsheet hold
 *   both: their text is pasted), goes where the selection is, which it
 *   replaces; an image without a name of its own is named as Obsidian
 *   names it (pastedName).
 * - A drop of files from outside the page goes where it is dropped; a
 *   drag started in the page (an image dragged carries a file in
 *   Chromium), and one of pages' files (.md) only, are CodeMirror's,
 *   which inserts their text. A folder is not uploaded: it says to
 *   import one. A read-only editor takes no file.
 * - Each embed is inserted once its upload is answered, where its files
 *   went, as the content changed since, in their order, a line each;
 *   once a composition ends. One failed or cancelled inserts nothing, its
 *   row saying why; one uploaded as the content was replaced, or can no
 *   longer be changed, or without an extension (no link), is not
 *   inserted, and the editor says so: it is in the page's attachments.
 */
export const assetUpload: Build = (context, controls) => {
  let closed = false;
  controls.onClose(() => {
    closed = true;
  });
  return [
    places,
    Prec.highest(
      EditorView.domEventHandlers({
        paste(event, view) {
          const transfer = event.clipboardData;
          if (transfer === null || transfer.types.includes("text/plain") || view.state.readOnly) {
            return false;
          }
          const { files } = filesDropped(transfer);
          if (files.length === 0) {
            return false;
          }
          const now = new Date();
          const named = files.map((file) => new File([file], pastedName(file, now), { type: file.type }));
          const { from, to } = view.state.selection.main;
          if (from !== to) {
            view.dispatch({ changes: { from, to }, selection: { anchor: from }, userEvent: "delete.cut" });
          }
          void upload(view, from, named, context, controls, () => closed);
          return true;
        },
        dragover(event, view) {
          if (!carriesFiles(event.dataTransfer) || view.state.readOnly) {
            return false;
          }
          if (event.dataTransfer !== null) {
            event.dataTransfer.dropEffect = "copy";
          }
          return true;
        },
        drop(event, view) {
          const transfer = event.dataTransfer;
          if (transfer === null || !carriesFiles(transfer) || view.state.readOnly) {
            return false;
          }
          const { files, folders } = filesDropped(transfer);
          if (files.length > 0 && files.every((file) => isPageName(file.name))) {
            return false;
          }
          if (folders) {
            controls.tell(view.state.phrase("Folders are not uploaded: import a folder of notes instead."));
          }
          const at = view.posAtCoords({ x: event.clientX, y: event.clientY }) ?? view.state.selection.main.head;
          void upload(view, at, files, context, controls, () => closed);
          return true;
        },
      })
    ),
  ];
};

/** What the editor says of an attachment uploaded and not inserted: the content was replaced, or can no longer be changed. */
const notInserted = "$ is in the page's attachments, not inserted: the text was replaced or can no longer be changed.";

/** What the editor says of an attachment uploaded without an extension: no link embeds it. */
const noLink = "$ is in the page's attachments, not inserted: a name without an extension cannot be embedded.";

/** How many places have been opened: each its number. */
let opened = 0;

/**
 * upload uploads files side by side, and inserts each one's embed, in
 * their order, at the place opened at at, while the editor's state is
 * the same (isClosed tells it went) and its content can be changed.
 */
async function upload(
  view: EditorView,
  at: number,
  files: readonly File[],
  context: EditorContext,
  controls: EditorControls,
  isClosed: () => boolean
): Promise<void> {
  if (files.length === 0) {
    return;
  }
  const id = ++opened;
  view.dispatch({ effects: placeAdded.of({ id, at }) });
  const uploads = files.map((file) => context.uploadAsset(file));
  let first = true;
  for (const going of uploads) {
    let asset: Asset;
    try {
      // oxlint-disable-next-line no-await-in-loop -- each embed after the one before, in their order
      asset = await going;
    } catch {
      // Its row says why.
      continue;
    }
    // oxlint-disable-next-line no-await-in-loop -- never in half a word
    const composed = await new Promise<boolean>((resolve) =>
      controls.whenComposed(
        () => resolve(true),
        () => resolve(false)
      )
    );
    const state = view.state;
    const place = composed && !isClosed() ? state.field(places, false)?.get(id) : undefined;
    if (asset.link === null) {
      controls.tell(state.phrase(noLink, asset.name));
    } else if (place === undefined || state.readOnly) {
      controls.tell(state.phrase(notInserted, asset.name));
    } else {
      const insert = (first ? "" : "\n") + embedOf(asset.link);
      view.dispatch({
        changes: { from: place, insert },
        effects: placeAdded.of({ id, at: place + insert.length }),
        userEvent: "input.paste",
      });
      first = false;
    }
  }
  if (!isClosed()) {
    view.dispatch({ effects: placeDone.of(id) });
  }
}
