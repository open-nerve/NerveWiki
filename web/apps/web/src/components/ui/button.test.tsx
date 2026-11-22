import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";

import { Button } from "./button";

// The buttons' looks (M13/P1 design 4.1.5, 4.2.2).

test("the outline button layers its hover on its own background", () => {
  render(<Button variant="outline">Cancel</Button>);
  const { classList } = screen.getByRole("button", { name: "Cancel" });
  expect(classList.contains("bg-surface")).toBe(true);
  expect(classList.contains("hover:hover-layer")).toBe(true);
  // Dark, a hover of its own in place of the background shows what is under the button, darker (decision 757).
  expect(classList.contains("hover:bg-hover")).toBe(false);
});
