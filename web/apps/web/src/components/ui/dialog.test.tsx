import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";

import { AlertDialog, AlertDialogContent, AlertDialogDescription, AlertDialogTitle } from "./alert-dialog";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./dialog";

// The dialogs' entrance (M13/P1 design 4.2.7, 4.2.9). jsdom runs no animation: the classes say what a browser would.

/** expectEntranceOnly expects of a dialog's part that it animates as it opens, and never as it closes. */
function expectEntranceOnly(part: Element, animation: string): void {
  expect(part.getAttribute("data-state")).toBe("open");
  expect(part.classList.contains(`data-[state=open]:${animation}`), animation).toBe(true);
  // An exit's animation would keep the closed dialog mounted until it ends (00-M13 737).
  expect(part.className).not.toContain("data-[state=closed]:");
}

/** overlay is the open dialog's overlay: the one other part its portal has. */
function overlay(): Element {
  const found = document.body.querySelectorAll(".bg-overlay");
  expect(found).toHaveLength(1);
  return found[0] as Element;
}

test("a dialog animates in, its overlay too, never out", () => {
  render(
    <Dialog open>
      <DialogContent>
        <DialogTitle>Rename</DialogTitle>
        <DialogDescription>A new name.</DialogDescription>
      </DialogContent>
    </Dialog>
  );
  expectEntranceOnly(screen.getByRole("dialog", { name: "Rename" }), "animate-pop-in");
  expectEntranceOnly(overlay(), "animate-fade-in");
});

test("an alert dialog animates in, its overlay too, never out", () => {
  render(
    <AlertDialog open>
      <AlertDialogContent>
        <AlertDialogTitle>Delete</AlertDialogTitle>
        <AlertDialogDescription>It cannot be undone.</AlertDialogDescription>
      </AlertDialogContent>
    </AlertDialog>
  );
  expectEntranceOnly(screen.getByRole("alertdialog", { name: "Delete" }), "animate-pop-in");
  expectEntranceOnly(overlay(), "animate-fade-in");
});
