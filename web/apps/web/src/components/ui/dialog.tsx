import { Dialog as Primitive } from "radix-ui";
import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

export const Dialog = Primitive.Root;
export const DialogTrigger = Primitive.Trigger;
export const DialogClose = Primitive.Close;

/**
 * DialogContent is a dialog's box. Closed, it gives the focus back where it was, as Radix does, the caller's
 * onCloseAutoFocus first; the keys held as it closes press nothing there (swallowHeldKeys).
 */
export function DialogContent({ className, onCloseAutoFocus, ...props }: ComponentProps<typeof Primitive.Content>) {
  return (
    <Primitive.Portal>
      <Primitive.Overlay className="fixed inset-0 z-50 bg-overlay data-[state=open]:animate-fade-in" />
      <Primitive.Content
        className={cn(
          "fixed top-1/2 left-1/2 z-50 grid w-full max-w-[calc(100%-2rem)] grid-cols-[minmax(0,1fr)] -translate-x-1/2 -translate-y-1/2 gap-4 rounded-dialog border border-border bg-surface p-6 shadow-dialog data-[state=open]:animate-pop-in sm:max-w-md",
          className
        )}
        onCloseAutoFocus={(event) => {
          swallowHeldKeys();
          onCloseAutoFocus?.(event);
        }}
        {...props}
      />
    </Primitive.Portal>
  );
}

export function DialogTitle({ className, ...props }: ComponentProps<typeof Primitive.Title>) {
  return <Primitive.Title className={cn("text-lg font-semibold [overflow-wrap:anywhere]", className)} {...props} />;
}

export function DialogDescription({ className, ...props }: ComponentProps<typeof Primitive.Description>) {
  return <Primitive.Description className={cn("text-row text-muted-foreground", className)} {...props} />;
}

/** Whether the repeats of a held key are being swallowed: once, however many dialogs close meanwhile. */
let swallowing = false;

/**
 * swallowHeldKeys has a key held as a dialog closes press nothing where the focus goes back to (M13 closeout, review
 * B's M13CB-M1, decision 967). Enter held on Cancel closes the dialog on its first press; its repeats then fell on the
 * button that opened it, which opened it again, or a tree's menu, whose first item, focused, made a page. Until a key
 * is let go, a repeat of Enter or Space does nothing: neither its default (a button's click) nor the page's listeners,
 * the capture on the document coming first. A new press of either ends it too, a dialog closed by the pointer leaving
 * the keys be: held, that one repeats as it would. A form's own guard of its buttons (the import's, decision 960)
 * stays.
 */
export function swallowHeldKeys(): void {
  if (swallowing) {
    return;
  }
  swallowing = true;
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    if (!event.repeat) {
      done();
      return;
    }
    event.preventDefault();
    event.stopPropagation();
  };
  const done = () => {
    swallowing = false;
    document.removeEventListener("keydown", onKeyDown, true);
    document.removeEventListener("keyup", done, true);
  };
  document.addEventListener("keydown", onKeyDown, true);
  document.addEventListener("keyup", done, true);
}
