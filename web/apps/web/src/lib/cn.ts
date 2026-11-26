import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

// The theme's radii, shadows, sizes and animations (styles.css), which tailwind-merge does not know otherwise: unknown, a
// component's rounded-row and its caller's rounded-full both stay, Tailwind's order of them deciding; text-row is
// taken for a colour, and a colour for it (M13/P1/S1, decision 756).
const merge = extendTailwindMerge({
  extend: {
    theme: {
      radius: ["row", "control", "popover", "card", "dialog"],
      shadow: ["popover", "dialog"],
      text: ["compact", "row"],
      animate: ["pop-in", "fade-in"],
    },
  },
});

/** cn joins class names; of two Tailwind classes that conflict, the later wins. */
export function cn(...classes: ClassValue[]): string {
  return merge(clsx(classes));
}
