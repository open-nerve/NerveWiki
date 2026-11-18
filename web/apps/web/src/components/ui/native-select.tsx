import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

/**
 * NativeSelect is the browser's select, styled as the inputs: its keyboard and its screen reader's reading are the
 * platform's. Its paint is contained: WebKit counts the text of the option a select shows, as long as it is, in the
 * page's width, though the select clips it (a notebook's long name made the search's results 704 wide at 375; M12/P2/S3,
 * M5-polish 33.1). Its own ring and border are not its content and stay.
 */
export function NativeSelect({ className, ...props }: ComponentProps<"select">) {
  return (
    <select
      className={cn(
        "h-9 max-w-full rounded-md border bg-background px-2 text-sm text-foreground shadow-xs outline-none transition-[color,box-shadow] contain-paint",
        "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
        className
      )}
      {...props}
    />
  );
}
