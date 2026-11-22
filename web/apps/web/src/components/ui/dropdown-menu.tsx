import { Check } from "lucide-react";
import { DropdownMenu as Primitive } from "radix-ui";
import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

export const DropdownMenu = Primitive.Root;
export const DropdownMenuTrigger = Primitive.Trigger;
export const DropdownMenuRadioGroup = Primitive.RadioGroup;

// A menu pops in from the side that opened it, and closes at once (M13/P1/S2; 00-M13 737). Its items' ring shows for
// the keyboard alone (app/input-modality.ts, decision 760); with the pointer the highlight is its background.

export function DropdownMenuContent({ className, sideOffset = 4, ...props }: ComponentProps<typeof Primitive.Content>) {
  return (
    <Primitive.Portal>
      <Primitive.Content
        sideOffset={sideOffset}
        className={cn(
          "z-50 min-w-[220px] origin-(--radix-dropdown-menu-content-transform-origin) overflow-hidden rounded-popover border border-border bg-popover p-1.25 text-popover-foreground shadow-popover data-[state=open]:animate-pop-in",
          className
        )}
        {...props}
      />
    </Primitive.Portal>
  );
}

const item =
  "relative flex h-8 cursor-default items-center gap-2.5 rounded-row text-row outline-none select-none focus:bg-active focus:text-foreground focus:ring-ring focus:ring-inset in-data-[modality=keyboard]:focus:ring-2 data-disabled:pointer-events-none data-disabled:opacity-50 [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-muted-foreground";

type ItemProps = ComponentProps<typeof Primitive.Item> & {
  /** destructive: the item deletes, in red. */
  tone?: "destructive";
};

export function DropdownMenuItem({ className, tone, ...props }: ItemProps) {
  return (
    <Primitive.Item
      className={cn(
        item,
        "px-2.5",
        tone === "destructive" && "text-destructive focus:text-destructive [&>svg]:text-destructive",
        className
      )}
      {...props}
    />
  );
}

export function DropdownMenuRadioItem({ className, children, ...props }: ComponentProps<typeof Primitive.RadioItem>) {
  return (
    <Primitive.RadioItem className={cn(item, "pr-2.5 pl-8", className)} {...props}>
      <span className="absolute left-2 flex size-3.5 items-center justify-center">
        <Primitive.ItemIndicator>
          <Check className="size-3.5" />
        </Primitive.ItemIndicator>
      </span>
      {children}
    </Primitive.RadioItem>
  );
}

export function DropdownMenuSeparator({ className, ...props }: ComponentProps<typeof Primitive.Separator>) {
  return <Primitive.Separator className={cn("mx-1.5 my-1 h-px bg-border", className)} {...props} />;
}

export function DropdownMenuLabel({ className, ...props }: ComponentProps<typeof Primitive.Label>) {
  return <Primitive.Label className={cn("px-2.5 py-1.5 text-xs text-faint", className)} {...props} />;
}
