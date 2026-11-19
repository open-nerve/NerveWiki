import { useRef, useState, type ChangeEvent, type ComponentProps, type KeyboardEvent } from "react";

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

/** The keys that step a closed select from one option to another, without Alt (Alt+Up or Down opens its list). */
const steps = new Set(["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Home", "End", "PageUp", "PageDown"]);

/** stepping tells whether a key pressed on a closed select steps it: a step's key, or a character typed to find one. */
function stepping(event: KeyboardEvent<HTMLSelectElement>): boolean {
  if (event.altKey || event.ctrlKey || event.metaKey) {
    return false;
  }
  return steps.has(event.key) || (event.key !== " " && [...event.key].length === 1);
}

/**
 * useChoice is a select's value and handlers for a choice whose use reads
 * something (a search, a list): applied is the choice in use, apply uses
 * another (M5-polish 26.5). Off macOS, the keys on a closed select step
 * from option to option, each step a change: each would read. A change the
 * keys stepped to waits, shown, until the reader leaves the select or
 * presses Enter; one from the pointer, the open list or a script is used
 * at once. Alt and an arrow, F4 and Space open the list: no step. What is
 * applied changing elsewhere (the address) drops what waits. Used, the
 * choice already in use reads nothing again.
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
