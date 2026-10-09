import { useEffect, useState, type DragEvent } from "react";

/**
 * pageDrag is whether a drag started in the page is going: an image of the
 * reading view dragged carries a file in Chromium, which is no file from
 * outside. FileDropGuard keeps it. A drag's end does not reach the
 * document where its source left it meanwhile, nor a drop where it was
 * cancelled; the pointer's next move or press, which no drag has, ends it
 * then. The page's data on the drag stays the browser's own: written to,
 * it would not be (WebKit), and it would go along to other tabs.
 */
const pageDrag = { on: false };

/** hasFiles tells whether a drag carries files, which the browser would open where nothing takes them. */
function hasFiles(transfer: DataTransfer | null): boolean {
  return transfer?.types.includes("Files") === true;
}

/** carriesFiles tells whether a drag carries files from outside the page, which a drop zone uploads. */
function carriesFiles(transfer: DataTransfer | null): boolean {
  return !pageDrag.on && hasFiles(transfer);
}

/** editable tells whether target is in an editor, which takes what is dropped on it as it does (M7/P4C). */
function editable(target: EventTarget | null): boolean {
  const element = target instanceof Element ? target : target instanceof Node ? target.parentElement : null;
  return element !== null && element.closest("[contenteditable=true]") !== null;
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
 * Over the document, a drag of files may not drop, one started in the page
 * (an image of it) or in another tab too; where one may drop, its handler
 * said so first (its dragover's default prevented), and an editor takes
 * them as it does. The shell mounts it once.
 */
export function FileDropGuard() {
  useEffect(() => {
    const onDragStart = () => {
      pageDrag.on = true;
    };
    const ended = () => {
      pageDrag.on = false;
    };
    const onDragOver = (event: globalThis.DragEvent) => {
      if (hasFiles(event.dataTransfer) && !event.defaultPrevented && !editable(event.target)) {
        event.preventDefault();
        if (event.dataTransfer !== null) {
          event.dataTransfer.dropEffect = "none";
        }
      }
    };
    const onDrop = (event: globalThis.DragEvent) => {
      if (hasFiles(event.dataTransfer) && !editable(event.target)) {
        event.preventDefault();
      }
      ended();
    };
    const ends = ["dragend", "pointermove", "pointerdown"] as const;
    document.addEventListener("dragstart", onDragStart);
    document.addEventListener("dragover", onDragOver);
    document.addEventListener("drop", onDrop);
    for (const type of ends) {
      document.addEventListener(type, ended, { capture: true, passive: true });
    }
    return () => {
      document.removeEventListener("dragstart", onDragStart);
      document.removeEventListener("dragover", onDragOver);
      document.removeEventListener("drop", onDrop);
      for (const type of ends) {
        document.removeEventListener(type, ended, true);
      }
      ended();
    };
  }, []);
  return null;
}

/** within tells whether a drag event is over the element of the handler it reached: one in a portal, a dialog, is not. */
function within(event: DragEvent): boolean {
  return event.target instanceof Node && event.currentTarget.contains(event.target);
}

/**
 * useFileDrop makes an element where files dropped go to take, while
 * enabled: a drag of files over it may drop (copy) and shows (over); the
 * drop gives drop its files, and whether it held a folder. A drag over a
 * dialog it renders, which React's events reach through the portal, is
 * not over it.
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
        if (!enabled || !carriesFiles(event.dataTransfer) || !within(event)) {
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
        if (!enabled || !carriesFiles(event.dataTransfer) || !within(event)) {
          return;
        }
        event.preventDefault();
        setOver(false);
        drop(filesDropped(event.dataTransfer));
      },
    },
  };
}
