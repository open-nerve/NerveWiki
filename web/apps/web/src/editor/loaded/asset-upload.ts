import { Prec, StateEffect, StateField, type EditorState } from "@codemirror/state";
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
 * number, in the order they were opened: where they were pasted or
 * dropped, mapped through each change since, before what is typed there
 * (the embed was pasted first); after each embed inserted at one, the
 * next of its files goes after it, and so do the places opened there
 * since (pasted after it).
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
  "image/x-icon": "ico",
  "image/vnd.microsoft.icon": "ico",
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
 * - A drop of files from outside the page goes where it is dropped (the
 *   editor shows where as they are dragged); a drag started in the page (an image dragged carries a file in Chromium), and one of
 *   pages' files (.md) only, are CodeMirror's, as is one that holds no
 *   file to read. A read-only editor takes no file.
 * - A folder pasted or dropped is not uploaded: the editor says to import
 *   one, with a drop's pages' files too.
 * - Each embed is inserted once its upload is answered, where its files
 *   went, as the content changed since, in their order, a line each, the
 *   cursor there going after it as after a paste; once a composition
 *   ends. One failed or cancelled inserts nothing, its row saying why;
 *   one uploaded as the content was replaced, or can no longer be
 *   changed, or without an extension (no link), is not inserted, and the
 *   editor says so of them all as the last of the files is answered, in
 *   place of what it said before; each paste or drop begins anew.
 * - The edit waits for them to be inserted before it is left (going).
 */
export const assetUpload: Build = (context, controls) => {
  let closed = false;
  controls.onClose(() => {
    closed = true;
  });
  /** take uploads files at at, saying first what is not: folders, or nothing. */
  const take = (view: EditorView, at: number, files: readonly File[], folders: boolean) => {
    controls.tell(folders ? view.state.phrase(foldersPhrase) : "");
    controls.going(
      upload(view, at, { files, folders }, context, controls, () => closed).catch((error: unknown) => {
        console.error("The files pasted or dropped could not be inserted", error);
      })
    );
  };
  return [
    places,
    Prec.highest(
      EditorView.domEventHandlers({
        paste(event, view) {
          const transfer = event.clipboardData;
          if (transfer === null || transfer.types.includes("text/plain") || view.state.readOnly) {
            return false;
          }
          const { files, folders } = filesDropped(transfer);
          if (files.length === 0 && !folders) {
            return false;
          }
          const now = new Date();
          const named = files.map((file) => new File([file], pastedName(file, now), { type: file.type }));
          const { from, to } = view.state.selection.main;
          if (from !== to && named.length > 0) {
            view.dispatch({ changes: { from, to }, selection: { anchor: from }, userEvent: "delete.cut" });
          }
          take(view, from, named, folders);
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
          if (files.length === 0 && !folders) {
            return false;
          }
          // Pages' files only: CodeMirror reads their text in.
          if (files.length > 0 && files.every((file) => isPageName(file.name))) {
            if (folders) {
              controls.tell(view.state.phrase(foldersPhrase));
            }
            return false;
          }
          const at = view.posAtCoords({ x: event.clientX, y: event.clientY }) ?? view.state.selection.main.head;
          take(view, at, files, folders);
          return true;
        },
      })
    ),
  ];
};

/** What the editor says of folders pasted or dropped: they are imported. */
const foldersPhrase = "Folders are not uploaded: import a folder of notes instead.";

/** What the editor says of attachments uploaded and not inserted: the content was replaced, or can no longer be changed. */
const notInserted = "$ uploaded, not inserted: the text was replaced or can no longer be changed.";

/** What the editor says of attachments uploaded without an extension: no link embeds them. */
const noLink = "$ uploaded, not inserted: a name without an extension cannot be embedded.";

/** How many places have been opened: each its number. */
let opened = 0;

/** namesOf lists names as the document's language does: a, b and c. */
function namesOf(names: readonly string[]): string {
  return new Intl.ListFormat(document.documentElement.lang || undefined, { type: "conjunction" }).format(names);
}

/** laterAt are the places opened after id that are where it is, at. */
function laterAt(state: EditorState, id: number, at: number): number[] {
  return [...(state.field(places, false) ?? [])]
    .filter(([other, place]) => other > id && place === at)
    .map(([other]) => other);
}

/**
 * upload uploads a paste's or a drop's files side by side, and inserts
 * each one's embed, in their order, at the place opened at at, while the
 * editor's state is the same (isClosed tells it went) and its content can
 * be changed; then it says what was not inserted, its folders too (in
 * place of what it said of them), a line each. Of pastes and drops going together,
 * what the last answered says stays.
 */
async function upload(
  view: EditorView,
  at: number,
  { files, folders }: { files: readonly File[]; folders: boolean },
  context: EditorContext,
  controls: EditorControls,
  isClosed: () => boolean
): Promise<void> {
  if (files.length === 0) {
    return;
  }
  const id = ++opened;
  view.dispatch({ effects: placeAdded.of({ id, at }) });
  // Each settles as it is answered, whatever the order: one failed or cancelled is undefined, its row saying why.
  const uploads = files.map((file) =>
    context.uploadAsset(file).then(
      (asset): Asset | undefined => asset,
      () => undefined
    )
  );
  const unlinked: string[] = [];
  const left: string[] = [];
  let first = true;
  for (const answered of uploads) {
    // oxlint-disable-next-line no-await-in-loop -- each embed after the one before, in their order
    const asset = await answered;
    if (asset === undefined) {
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
      unlinked.push(asset.name);
    } else if (place === undefined || state.readOnly) {
      left.push(asset.name);
    } else {
      const changes = state.changes({ from: place, insert: (first ? "" : "\n") + embedOf(asset.link) });
      const after = changes.mapPos(place, 1);
      // The cursor where it goes goes after it, as after a paste; non-empty, a selection keeps what it holds. Set only
      // then: a selection set resets what follows it, an open completion.
      const moved = state.selection.map(changes, 1);
      view.dispatch({
        changes,
        ...(moved.eq(state.selection.map(changes)) ? {} : { selection: moved }),
        effects: [id, ...laterAt(state, id, place)].map((each) => placeAdded.of({ id: each, at: after })),
        userEvent: "input.paste",
      });
      first = false;
    }
  }
  if (!isClosed()) {
    view.dispatch({ effects: placeDone.of(id) });
  }
  const said = [
    folders && (unlinked.length > 0 || left.length > 0) ? view.state.phrase(foldersPhrase) : "",
    unlinked.length > 0 ? view.state.phrase(noLink, namesOf(unlinked)) : "",
    left.length > 0 ? view.state.phrase(notInserted, namesOf(left)) : "",
  ].filter((sentence) => sentence !== "");
  if (said.length > 0) {
    controls.tell(said.join("\n"));
  }
}
