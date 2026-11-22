import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

/** Alert is a message about the page or the form, announced as it appears. */
export function Alert({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      role="alert"
      className={cn(
        "rounded-lg border border-destructive/35 bg-destructive/5 px-4 py-3 text-sm text-destructive",
        className
      )}
      {...props}
    />
  );
}
