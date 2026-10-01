import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

/** NativeSelect is the browser's select, styled as the inputs: its keyboard and its screen reader's reading are the platform's. */
export function NativeSelect({ className, ...props }: ComponentProps<"select">) {
  return (
    <select
      className={cn(
        "h-9 rounded-md border bg-background px-2 text-sm text-foreground shadow-xs outline-none transition-[color,box-shadow]",
        "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50",
        className
      )}
      {...props}
    />
  );
}
