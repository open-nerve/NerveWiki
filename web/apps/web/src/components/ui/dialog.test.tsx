import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "./alert-dialog";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "./dialog";

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

// M13/P5/S2 (decision 952): the one column of either is the dialog's width, so that what is in it wraps inside it, as
// an alert naming a page and who edits it does, its buttons kept in the dialog.
test.each([
  [
    "dialog",
    () => (
      <Dialog open>
        <DialogContent>
          <DialogTitle>Rename</DialogTitle>
          <DialogDescription>A new name.</DialogDescription>
        </DialogContent>
      </Dialog>
    ),
  ],
  [
    "alertdialog",
    () => (
      <AlertDialog open>
        <AlertDialogContent>
          <AlertDialogTitle>Delete</AlertDialogTitle>
          <AlertDialogDescription>It cannot be undone.</AlertDialogDescription>
        </AlertDialogContent>
      </AlertDialog>
    ),
  ],
] as const)("the %s's one column is no wider than it", (role, dialog) => {
  render(dialog());
  expect(screen.getByRole(role).classList.contains("grid-cols-[minmax(0,1fr)]")).toBe(true);
});

// M13 closeout (decision 967; review B's M13CB-M1): Enter held on a dialog's Cancel closes it, and its repeats fall on
// the button the focus goes back to, which opened it: it opened again, and a tree's menu, its first item focused,
// made a page. Until a key is let go, a repeat of Enter or Space does nothing, whichever dialog closed; a press after
// one closed by the pointer, held, repeats as it would.
test.each([
  [
    "dialog",
    (closed: () => void) => (
      <Dialog>
        <DialogTrigger>Open</DialogTrigger>
        <DialogContent onCloseAutoFocus={closed}>
          <DialogTitle>Rename</DialogTitle>
          <DialogDescription>A new name.</DialogDescription>
          <DialogClose>Cancel</DialogClose>
        </DialogContent>
      </Dialog>
    ),
  ],
  [
    "alertdialog",
    (closed: () => void) => (
      <AlertDialog>
        <AlertDialogTrigger>Open</AlertDialogTrigger>
        <AlertDialogContent onCloseAutoFocus={closed}>
          <AlertDialogTitle>Delete</AlertDialogTitle>
          <AlertDialogDescription>It cannot be undone.</AlertDialogDescription>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
        </AlertDialogContent>
      </AlertDialog>
    ),
  ],
] as const)(
  "Enter held on the %s's Cancel closes it, its repeats pressing nothing on the button the focus goes back to; let go, Enter opens it again",
  async (role, dialog) => {
    const user = userEvent.setup();
    const closed = vi.fn();
    render(dialog(closed));
    const trigger = screen.getByRole("button", { name: "Open" });
    await user.click(trigger);
    screen.getByRole("button", { name: "Cancel" }).focus();

    await user.keyboard("{Enter>4}");
    expect(screen.queryByRole(role)).toBeNull();
    expect(document.activeElement).toBe(trigger);
    // The dialog's own onCloseAutoFocus is called still.
    expect(closed).toHaveBeenCalledTimes(1);

    await user.keyboard("{/Enter}{Enter}");
    expect(screen.getByRole(role)).toBeTruthy();
  }
);

test("a dialog closed by the pointer leaves the keys be: Enter pressed next and held repeats as it would", async () => {
  const user = userEvent.setup();
  const pressed = vi.fn();
  render(
    <>
      <button type="button" onClick={pressed}>
        Count
      </button>
      <Dialog>
        <DialogTrigger>Open</DialogTrigger>
        <DialogContent>
          <DialogTitle>Rename</DialogTitle>
          <DialogDescription>A new name.</DialogDescription>
          <DialogClose>Cancel</DialogClose>
        </DialogContent>
      </Dialog>
    </>
  );
  await user.click(screen.getByRole("button", { name: "Open" }));
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(screen.queryByRole("dialog")).toBeNull();

  screen.getByRole("button", { name: "Count" }).focus();
  await user.keyboard("{Enter>3/}");
  expect(pressed).toHaveBeenCalledTimes(3);
});
