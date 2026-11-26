import { AlertDialog as Primitive } from "radix-ui";
import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";
import { swallowHeldKeys } from "./dialog";

// A dialog that asks to confirm what cannot be undone: it keeps the focus,
// and only its buttons close it (not a click outside). Its one column is
// its width, as a dialog's is: what is in it wraps inside it (decision 952).
// Closed, the keys held press nothing where the focus goes back to, as a
// dialog's (swallowHeldKeys, decision 967).

export const AlertDialog = Primitive.Root;
export const AlertDialogTrigger = Primitive.Trigger;
export const AlertDialogCancel = Primitive.Cancel;

export function AlertDialogContent({
  className,
  onCloseAutoFocus,
  ...props
}: ComponentProps<typeof Primitive.Content>) {
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

export function AlertDialogTitle({ className, ...props }: ComponentProps<typeof Primitive.Title>) {
  return <Primitive.Title className={cn("text-lg font-semibold [overflow-wrap:anywhere]", className)} {...props} />;
}

export function AlertDialogDescription({ className, ...props }: ComponentProps<typeof Primitive.Description>) {
  return <Primitive.Description className={cn("text-row text-muted-foreground", className)} {...props} />;
}
