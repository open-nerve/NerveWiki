import { readFileSync } from "node:fs";
import { URL as FileURL } from "node:url";
import { expect, test } from "vitest";

import { cn } from "./cn";

// The theme's radii, shadows, sizes and animations are tailwind-merge's too (M13/P1/S1, decision 756; the
// animations M13/P1/S2): of a component's class and its caller's, the caller's wins; a size of the theme is no
// colour, to take a colour's place.

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

test("an animation of the theme gives way to the caller's", () => {
  expect(cn("data-[state=open]:animate-pop-in", "data-[state=open]:animate-none")).toBe(
    "data-[state=open]:animate-none"
  );
});

test("every radius, shadow, size and animation the theme declares is tailwind-merge's", () => {
  // Read from the disk: vitest serves a stylesheet's ?raw as "" (P1 design 1). Tailwind's own md and lg among the
  // radii tailwind-merge knows already.
  const theme = [
    ...readFileSync(new FileURL("../styles.css", import.meta.url), "utf8").matchAll(
      /@theme(?:\s+inline)?\s*\{([^}]*)\}/g
    ),
  ]
    .map(([, body]) => body ?? "")
    .join("\n");
  const names = (prefix: string) =>
    [...theme.matchAll(new RegExp(`${prefix}([\\w-]+)\\s*:`, "g"))].map(([, n]) => n ?? "");
  const radii = names("--radius-");
  const shadows = names("--shadow-");
  const sizes = names("--text-");
  const animations = names("--animate-");
  expect(radii).toEqual(expect.arrayContaining(["row", "control", "popover", "card", "dialog"]));
  expect(shadows).toEqual(expect.arrayContaining(["popover", "dialog"]));
  expect(sizes).toEqual(expect.arrayContaining(["compact", "row"]));
  expect(animations).toEqual(expect.arrayContaining(["pop-in", "fade-in"]));
  for (const radius of radii) expect(cn(`rounded-${radius}`, "rounded-full"), radius).toBe("rounded-full");
  for (const shadow of shadows) expect(cn(`shadow-${shadow}`, "shadow-none"), shadow).toBe("shadow-none");
  for (const animation of animations) {
    expect(cn(`animate-${animation}`, "animate-none"), animation).toBe("animate-none");
  }
  for (const size of sizes) {
    expect(cn("text-sm", `text-${size}`), size).toBe(`text-${size}`);
    expect(cn(`text-${size}`, "text-muted-foreground"), size).toBe(`text-${size} text-muted-foreground`);
  }
});
