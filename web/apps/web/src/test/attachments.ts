import { screen, within } from "@testing-library/react";

import { assetNode, guide, install, linux, notes } from "./page-server";
import type { FakeTransfer } from "./transfer";

// The attachments' section of a page, as its tests find it (M7/P4 design 3.5, 3.6).

export const photo = assetNode(70, "photo.png", guide);
const manual = assetNode(71, "manual.pdf", guide);
export const archive = assetNode(72, "data.zip", guide);
const readme = assetNode(73, "README", guide);

/** nodes are Plans' pages, and Guide's attachments: a picture, a PDF, a file to download, and one no link leads to. */
export const nodes = [guide, install, linux, notes, photo, manual, archive, readme];

/** attachments waits for the section of the attachments shown. */
export function attachments(): Promise<HTMLElement> {
  return screen.findByRole("region", { name: "Attachments" });
}

/** rows are the names of the attachments listed, as their links read. */
export function rows(section: HTMLElement): string[] {
  const list = within(section)
    .queryAllByRole("list")
    .find((each) => each.getAttribute("aria-label") === null);
  return list === undefined ? [] : [...list.querySelectorAll(":scope > li > a")].map((link) => link.textContent);
}

/** picker is the section's file input, which Upload opens. */
export function picker(section: HTMLElement): HTMLInputElement {
  const input = section.querySelector<HTMLInputElement>("input[type=file]");
  if (input === null) {
    throw new Error("the section has no file input");
  }
  return input;
}

/**
 * dropped is a drag's transfer of files, and of folders, which only say
 * what they are; its drop effect, the browser's own, until a handler sets
 * it; a type set as it starts is among its types.
 */
export function dropped(files: File[], folders: string[] = []) {
  const types = ["Files"];
  return {
    types,
    setData: (type: string) => {
      if (!types.includes(type)) {
        types.push(type);
      }
    },
    files,
    items: [
      ...files.map((file) => ({
        kind: "file",
        getAsFile: () => file,
        webkitGetAsEntry: () => ({ isDirectory: false }),
      })),
      ...folders.map((name) => ({
        kind: "file",
        getAsFile: () => new File([], name),
        webkitGetAsEntry: () => ({ isDirectory: true }),
      })),
    ],
    dropEffect: "move",
  };
}

/** transfers are the uploads' transfers of a test's app (transferTo). */
export function transfers(app: { transfer?: unknown }): FakeTransfer[] {
  return (app.transfer as { made: FakeTransfer[] }).made;
}
