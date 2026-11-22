import { render, screen } from "@testing-library/react";
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
