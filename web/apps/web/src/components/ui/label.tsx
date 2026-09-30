import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

/** Label names the control whose id is htmlFor. */
export function Label({ className, htmlFor, ...props }: ComponentProps<"label"> & { htmlFor: string }) {
  return (
    <label htmlFor={htmlFor} className={cn("text-sm leading-none font-medium select-none", className)} {...props} />
  );
}
