import { readFileSync } from "node:fs";
import { URL as FileURL } from "node:url";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { Button } from "./button";

// The buttons' looks (M13/P1 design 4.1.5, 4.2.2).

test("the outline button layers its hover on its own background", () => {
  render(<Button variant="outline">Cancel</Button>);
  const { classList } = screen.getByRole("button", { name: "Cancel" });
  expect(classList.contains("bg-surface")).toBe(true);
  // Dark, on the secondary surface: in a dialog, whose surface it would be otherwise.
  expect(classList.contains("dark:bg-surface-2")).toBe(true);
  expect(classList.contains("hover:hover-layer")).toBe(true);
  // Dark, a hover of its own in place of the background shows what is under the button, darker (decision 757).
  expect(classList.contains("hover:bg-hover")).toBe(false);
});

test("every button's focus is amber, the destructive one's too", () => {
  // The ring says where the focus is, not what is dangerous (decision 749).
  for (const variant of ["default", "outline", "ghost", "destructive"] as const) {
    const { unmount } = render(<Button variant={variant}>{variant}</Button>);
    const { classList } = screen.getByRole("button", { name: variant });
    expect(classList.contains("focus-visible:ring-ring"), variant).toBe(true);
    expect(classList.contains("focus-visible:ring-destructive"), variant).toBe(false);
    unmount();
  }
});

test("the diff's buttons, which copy the outline button's look, keep to it", () => {
  // The editor does not import the app's button (text-diff.tsx's buttonClass): its copy is read as written.
  const source = readFileSync(new FileURL("../../editor/text-diff.tsx", import.meta.url), "utf8");
  const [, copied = ""] = /const buttonClass =\s*"([^"]*)"/.exec(source) ?? [];
  const classes = copied.split(/\s+/);
  for (const look of ["bg-surface", "dark:bg-surface-2", "hover:hover-layer", "focus-visible:ring-2"]) {
    expect(classes, look).toContain(look);
  }
  // Dark, a hover in place of its background would show what is under the button, darker (decision 757).
  expect(classes).not.toContain("hover:bg-hover");
});

test("each variant has its look", () => {
  // What tells one from another (M13/P1 design 4.2.2): the ink fill, the outline on its surface, the quiet ghost, the red
  // text that fills nothing.
  const looks = {
    default: ["bg-primary", "text-primary-foreground"],
    outline: ["bg-surface", "hover:hover-layer"],
    ghost: ["text-muted-foreground", "hover:bg-hover"],
    destructive: ["text-destructive", "border-destructive/35"],
  } as const;
  for (const [variant, classes] of Object.entries(looks)) {
    const { unmount } = render(<Button variant={variant as keyof typeof looks}>{variant}</Button>);
    const { classList } = screen.getByRole("button", { name: variant });
    for (const look of classes) expect(classList.contains(look), `${variant} ${look}`).toBe(true);
    unmount();
  }
  render(<Button variant="destructive">Delete</Button>);
  const destructive = screen.getByRole("button", { name: "Delete" });
  expect([...destructive.classList].filter((name) => name.startsWith("bg-destructive"))).toEqual([]);
});

test("an aria-disabled button stays in the tab order, a disabled one is left out", async () => {
  const user = userEvent.setup();
  render(
    <>
      <Button>Before</Button>
      <Button aria-disabled>Saving</Button>
      <Button disabled>Off</Button>
      <Button>After</Button>
    </>
  );
  // Busy, a button still takes the focus, which a screen reader then reads it with (v0.1 design 13.2 item 26).
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Before" }));
  await user.tab();
  const saving = screen.getByRole("button", { name: "Saving" });
  expect(document.activeElement).toBe(saving);
  // Nor is the pointer kept from it, jsdom applying no class: only its icons' and the disabled one's.
  expect(
    [...saving.classList].filter((name) => name.endsWith("pointer-events-none")),
    "pointer-events-none"
  ).toEqual(["disabled:pointer-events-none", "[&_svg]:pointer-events-none"]);
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "After" }));
});
