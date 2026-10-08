import { useEffect, useState, type DragEvent } from "react";

/** carriesFiles tells whether a drag carries files from outside the page, which the browser would open. */
function carriesFiles(transfer: DataTransfer | null): boolean {
  return transfer?.types.includes("Files") === true;
}

/**
 * filesDropped are the files of a drop, and whether it held a folder too,
 * which is not uploaded (M7/P4 design 3.6): a folder is imported. They are
 * read as the drop is handled: its items are gone after.
 */
function filesDropped(transfer: DataTransfer): { files: File[]; folders: boolean } {
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

/**
 * FileDropGuard keeps a file dropped outside where the app takes files from
 * leaving the app (M7/P4 design 3.6): the browser would open it in the tab.
 * Over the document, a drag of files may not drop; where one may drop, its
 * handler said so first (its dragover's default prevented). The shell
 * mounts it once.
 */
export function FileDropGuard() {
  useEffect(() => {
    const onDragOver = (event: globalThis.DragEvent) => {
      if (carriesFiles(event.dataTransfer) && !event.defaultPrevented) {
        event.preventDefault();
        if (event.dataTransfer !== null) {
          event.dataTransfer.dropEffect = "none";
        }
      }
    };
    const onDrop = (event: globalThis.DragEvent) => {
      if (carriesFiles(event.dataTransfer)) {
        event.preventDefault();
      }
    };
    document.addEventListener("dragover", onDragOver);
    document.addEventListener("drop", onDrop);
    return () => {
      document.removeEventListener("dragover", onDragOver);
      document.removeEventListener("drop", onDrop);
    };
  }, []);
  return null;
}

/**
 * useFileDrop makes an element where files dropped go to take, while
 * enabled: a drag of files over it may drop (copy) and shows (over); the
 * drop gives drop its files, and whether it held a folder.
 */
export function useFileDrop(
  enabled: boolean,
  drop: (dropped: { files: File[]; folders: boolean }) => void
): {
  over: boolean;
  handlers: {
    onDragOver: (event: DragEvent) => void;
    onDragLeave: (event: DragEvent) => void;
    onDrop: (event: DragEvent) => void;
  };
} {
  const [over, setOver] = useState(false);
  if (!enabled && over) {
    setOver(false);
  }
  return {
    over,
    handlers: {
      onDragOver: (event) => {
        if (!enabled || !carriesFiles(event.dataTransfer)) {
          return;
        }
        event.preventDefault();
        event.dataTransfer.dropEffect = "copy";
        setOver(true);
      },
      onDragLeave: (event) => {
        // Left for one of its own elements is no leaving.
        if (!(event.relatedTarget instanceof Node && event.currentTarget.contains(event.relatedTarget))) {
          setOver(false);
        }
      },
      onDrop: (event) => {
        if (!enabled || !carriesFiles(event.dataTransfer)) {
          return;
        }
        event.preventDefault();
        setOver(false);
        drop(filesDropped(event.dataTransfer));
      },
    },
  };
}
