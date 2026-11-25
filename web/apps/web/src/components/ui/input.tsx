import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

/**
 * A text field's look, an input's and a textarea's: its ground, border, corners, focus and invalid state (M13/P5/S3,
 * decision 934). Its focus has a transparent outline too, which forced colours draw (v0.1 design 13.2, item 31).
 */
export const fieldClass = cn(
  "w-full min-w-0 rounded-control border border-border bg-surface-2 px-3 text-base text-foreground outline-none transition-[color,background-color,border-color,box-shadow] duration-150 md:text-sm",
  "placeholder:text-faint focus-visible:border-ring focus-visible:bg-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden",
  "disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive"
);

export function Input({ className, ...props }: ComponentProps<"input">) {
  return (
    <input
      className={cn(
        "h-9 py-1",
        fieldClass,
        "file:mr-3 file:rounded-md file:border file:border-border file:bg-surface file:px-2 file:font-medium file:hover:hover-layer dark:file:bg-active",
        className
      )}
      {...props}
    />
  );
}
