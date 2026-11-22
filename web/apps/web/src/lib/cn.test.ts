import { expect, test } from "vitest";

import { cn } from "./cn";

// The theme's radii, shadows and sizes are tailwind-merge's too (M13/P1/S1, decision 756): of a component's class
// and its caller's, the caller's wins; a size of the theme is no colour, to take a colour's place.

test("a radius of the theme gives way to the caller's", () => {
  expect(cn("rounded-row", "rounded-full")).toBe("rounded-full");
  expect(cn("rounded-control", "rounded-lg")).toBe("rounded-lg");
});

test("a shadow of the theme gives way to the caller's", () => {
  expect(cn("shadow-popover", "shadow-none")).toBe("shadow-none");
});

test("a size of the theme is a size, not a colour", () => {
  expect(cn("text-row", "text-muted-foreground")).toBe("text-row text-muted-foreground");
  expect(cn("text-sm", "text-compact")).toBe("text-compact");
});
