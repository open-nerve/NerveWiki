import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/** cn joins class names; of two Tailwind classes that conflict, the later wins. */
export function cn(...classes: ClassValue[]): string {
  return twMerge(clsx(classes));
}
