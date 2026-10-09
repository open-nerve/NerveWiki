// What a drag or a paste carries of files (M7/P4 design 3.6, 5.2): the page's drop zones, its guard and the editor
// read it alike.

/**
 * pageDrag is whether a drag started in the page is going: an image of the
 * reading view dragged carries a file in Chromium, which is no file from
 * outside. FileDropGuard (app/file-drop.tsx) keeps it. A drag's end does not reach the
 * document where its source left it meanwhile, nor a drop where it was
 * cancelled; the pointer's next press, or move with no button down, which
 * no drag has, ends it then (Firefox moves the pointer, its button down,
 * as a drag begins). The page's data on the drag stays the browser's own: written to,
 * it would not be (WebKit), and it would go along to other tabs.
 */
export const pageDrag = { on: false };

/** hasFiles tells whether a drag carries files, which the browser would open where nothing takes them. */
export function hasFiles(transfer: DataTransfer | null): boolean {
  return transfer?.types.includes("Files") === true;
}

/** carriesFiles tells whether a drag carries files from outside the page, which a drop zone, or the editor, uploads. */
export function carriesFiles(transfer: DataTransfer | null): boolean {
  return !pageDrag.on && hasFiles(transfer);
}

/**
 * filesDropped are the files of a drop, or a paste, and whether it held a
 * folder too, which is not uploaded (M7/P4 design 3.6): a folder is
 * imported. They are read as the event is handled: its items are gone
 * after.
 */
export function filesDropped(transfer: DataTransfer): { files: File[]; folders: boolean } {
  const files: File[] = [];
  let folders = false;
  for (const item of transfer.items) {
    if (item.kind !== "file") {
      continue;
    }
    if (item.webkitGetAsEntry?.()?.isDirectory === true) {
      folders = true;
      continue;
    }
    const file = item.getAsFile();
    if (file !== null) {
      files.push(file);
    }
  }
  return { files, folders };
}
