import { useRef, useState, type ChangeEvent, type ComponentProps, type KeyboardEvent } from "react";

import { onMac } from "../../app/shortcuts";
import { cn } from "../../lib/cn";

/**
 * NativeSelect is the browser's select, styled as the inputs: its keyboard and its screen reader's reading are the
 * platform's. Its chevron is the app's (styles.css's nw-select: appearance none, the browser's own under forced
 * colours; M13/P1/S2). Its paint is contained: WebKit counts the text of the option a select shows, as long as it is,
 * in the page's width, though the select clips it (a notebook's long name made the search's results 704 wide at 375;
 * M12/P2/S3, M5-polish 33.1). Its own ring and border are not its content and stay.
 */
export function NativeSelect({ className, ...props }: ComponentProps<"select">) {
  return (
    <select
      className={cn(
        "nw-select h-9 max-w-full rounded-control border border-border bg-surface-2 pl-3 text-base text-foreground outline-none transition-[color,background-color,border-color,box-shadow] duration-150 contain-paint md:text-sm",
        "focus-visible:border-ring focus-visible:bg-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden disabled:pointer-events-none disabled:opacity-50",
        "aria-invalid:border-destructive aria-invalid:ring-destructive",
        className
      )}
      {...props}
    />
  );
}

/**
 * The keys that step a closed select from one option to another off macOS, without Alt, Ctrl or Meta (with Alt, Up
 * or Down opens its list where the platform does).
 */
const steps = new Set(["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Home", "End", "PageUp", "PageDown"]);

/**
 * stepping tells whether a key pressed on a closed select steps it: a character typed to find an option, or off
 * macOS a step's key. On macOS those open the list (inferred, not verified), whose choice is used at once: they are
 * no step there (M12/P2/S6 review B, Q1).
 */
function stepping(event: KeyboardEvent<HTMLSelectElement>): boolean {
  if (event.altKey || event.ctrlKey || event.metaKey) {
    return false;
  }
  return (event.key !== " " && [...event.key].length === 1) || (steps.has(event.key) && !onMac());
}

/**
 * useChoice is a select's value and handlers for a choice whose use reads
 * something (a search, a list): applied is the choice in use, apply uses
 * another (M5-polish 26.5). On Linux (Windows inferred, not verified), the
 * keys on a closed select step from option to option, each step a change:
 * each would read. A change the keys stepped to waits, shown, until the
 * reader leaves the select or presses Enter, which then opens no list, as
 * Chromium's would; one from the pointer, the open list or a script is used
 * at once. Alt and an arrow, F4 and Space are no step: they open the list
 * where the platform does (WebKit on Linux steps on Alt and an arrow, each
 * change used at once). On macOS the keys typed to find an option alone
 * step (stepping). What is applied changing elsewhere (the address) drops
 * what waits. Used, the choice already in use reads nothing again.
 */
export function useChoice(applied: string, apply: (value: string) => void) {
  // The option the keys stepped to, shown until it is used: a controlled select would otherwise show applied again.
  const [pending, setPending] = useState<string>();
  // Whether the change to come is the keys' step.
  const stepped = useRef(false);
  const [was, setWas] = useState(applied);
  if (was !== applied) {
    setWas(applied);
    setPending(undefined);
  }
  const use = (value: string) => {
    setPending(undefined);
    if (value !== applied) {
      apply(value);
    }
  };
  return {
    value: pending ?? applied,
    onKeyDown: (event: KeyboardEvent<HTMLSelectElement>) => {
      if (event.key === "Enter") {
        stepped.current = false;
        if (pending !== undefined) {
          // Not the list too, which Chromium opens on Enter (M12/P2/S6 review B, M1).
          event.preventDefault();
          use(pending);
        }
        return;
      }
      stepped.current = stepping(event);
    },
    onPointerDown: () => {
      stepped.current = false;
    },
    onChange: (event: ChangeEvent<HTMLSelectElement>) => {
      const value = event.target.value;
      if (stepped.current) {
        setPending(value);
      } else {
        use(value);
      }
      stepped.current = false;
    },
    onBlur: () => {
      if (pending !== undefined) {
        use(pending);
      }
    },
  };
}
