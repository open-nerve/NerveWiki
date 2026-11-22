import { Check, ChevronRight } from "lucide-react";
import { DropdownMenu as Primitive } from "radix-ui";
import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

export const DropdownMenu = Primitive.Root;
export const DropdownMenuTrigger = Primitive.Trigger;
export const DropdownMenuRadioGroup = Primitive.RadioGroup;

// A menu pops in from the side that opened it, and closes at once (M13/P1/S2; 00-M13 737). Its items' ring shows for
// the keyboard alone (app/input-modality.ts, decision 760); with the pointer the highlight is its background. Forced
// colours drop the ring and the background: a transparent outline, which they show, says where the focus is.

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
  "relative flex h-8 cursor-default items-center gap-2.5 rounded-row text-row outline-none select-none focus:bg-active focus:text-foreground focus:ring-ring focus:ring-inset in-data-[modality=keyboard]:focus:ring-2 focus-visible:outline-hidden data-disabled:pointer-events-none data-disabled:opacity-50 [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-muted-foreground";

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

// A submenu (M13/P2/S1, decision 773): the account's Appearance. Its trigger's arrow is a direct icon of the item, whose
// [&>svg]: classes (0,1,1) would size and colour it as the item's own; [&>svg:last-child]: (0,2,1) makes it smaller and
// fainter, tailwind-merge keeping both as different variants. A click opens it, the focus staying on its trigger; Enter,
// Space and Right open it at its first item; Left and Escape close it, Escape the whole menu.

export const DropdownMenuSub = Primitive.Sub;

export function DropdownMenuSubTrigger({ className, children, ...props }: ComponentProps<typeof Primitive.SubTrigger>) {
  return (
    <Primitive.SubTrigger
      className={cn(
        item,
        "px-2.5 data-[state=open]:bg-active [&>svg:last-child]:size-3.5 [&>svg:last-child]:text-faint",
        className
      )}
      {...props}
    >
      {children}
      <ChevronRight className="ml-auto" />
    </Primitive.SubTrigger>
  );
}

/**
 * DropdownMenuSubContent is a submenu, 8px off the window's edges unless it says otherwise: Radix's 0 had the account
 * row's Appearance, at the window's foot, cut by its bottom edge (M13/P2/S1; review B, S1B-M1).
 */
export function DropdownMenuSubContent({
  className,
  collisionPadding = 8,
  ...props
}: ComponentProps<typeof Primitive.SubContent>) {
  return (
    <Primitive.Portal>
      <Primitive.SubContent
        sideOffset={6}
        collisionPadding={collisionPadding}
        className={cn(
          "z-50 min-w-[200px] origin-(--radix-dropdown-menu-content-transform-origin) overflow-hidden rounded-popover border border-border bg-popover p-1.25 text-popover-foreground shadow-popover data-[state=open]:animate-pop-in",
          className
        )}
        {...props}
      />
    </Primitive.Portal>
  );
}

export function DropdownMenuSeparator({ className, ...props }: ComponentProps<typeof Primitive.Separator>) {
  // A border, not a background: forced colours draw it (review B, S1B-N5).
  return <Primitive.Separator className={cn("mx-1.5 my-1 h-0 border-t border-border", className)} {...props} />;
}

export function DropdownMenuLabel({ className, ...props }: ComponentProps<typeof Primitive.Label>) {
  return <Primitive.Label className={cn("px-2.5 py-1.5 text-xs text-faint", className)} {...props} />;
}
