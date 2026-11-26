import { render, screen } from "@testing-library/react";
import { Book } from "lucide-react";
import { expect, test } from "vitest";

import { EmptyState } from "./empty-state";

// An empty list said (M13/P5/S1, decision 913): an icon in its tile, read by no one, the line that says it, and what to
// do, if anything; no role: it is no status, what says it the pages' own outputs.

test.each(["page", "card"] as const)(
  "a %s's empty state: its icon in a tile read by no one, then its line, then what to do",
  (size) => {
    const { container } = render(
      <EmptyState icon={Book} size={size} action={<button type="button">New notebook</button>}>
        No notebooks yet.
      </EmptyState>
    );

    const root = container.firstElementChild as HTMLElement;
    const [tile, line, actions] = Array.from(root.children);
    expect(tile?.getAttribute("aria-hidden")).toBe("true");
    expect(tile?.querySelector("svg")).not.toBeNull();
    expect(tile?.textContent).toBe("");
    expect(line?.tagName).toBe("P");
    expect(line?.textContent).toBe("No notebooks yet.");
    expect(screen.getByText("No notebooks yet.", { selector: "p" })).toBe(line);
    expect(actions?.contains(screen.getByRole("button", { name: "New notebook" }))).toBe(true);
    expect(root.children).toHaveLength(3);
    expect(screen.queryByRole("status")).toBeNull();
    expect(root.getAttribute("role")).toBeNull();
  }
);

test.each(["page", "card"] as const)(
  "a %s's empty state with nothing to do has its tile and its line alone",
  (size) => {
    const { container } = render(
      <EmptyState icon={Book} size={size}>
        No pages yet.
      </EmptyState>
    );

    const root = container.firstElementChild as HTMLElement;
    expect(Array.from(root.children).map((child) => child.tagName)).toEqual(["DIV", "P"]);
    expect(root.firstElementChild?.getAttribute("aria-hidden")).toBe("true");
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
  }
);

test("a page's empty state and a card's differ in their size alone", () => {
  const { container: page } = render(<EmptyState icon={Book}>No pages yet.</EmptyState>);
  const { container: card } = render(
    <EmptyState icon={Book} size="card">
      No pages yet.
    </EmptyState>
  );

  // The page's by default: more room above and below, a larger tile.
  expect(page.firstElementChild?.className).toContain("py-14");
  expect(page.firstElementChild?.firstElementChild?.className).toContain("size-11");
  expect(card.firstElementChild?.className).toContain("py-7");
  expect(card.firstElementChild?.firstElementChild?.className).toContain("size-9");
});
