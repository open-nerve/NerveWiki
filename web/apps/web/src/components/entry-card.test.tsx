import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";

import { appName, useDocumentTitle } from "../app/document-title";
import { EntryCard } from "./entry-card";

// The card of the entry pages (M13/P5/S1, decision 920): the product's mark and name at its top, in text, then the
// page's own content.

/** A page in the card: its title, then a line. */
function Entry() {
  useDocumentTitle("Sign in");
  return (
    <EntryCard>
      <h1>Sign in</h1>
      <p>The form</p>
    </EntryCard>
  );
}

test("the card says the product's name above the page, in text, its mark read by no one; the tab's name ends with the same name", () => {
  const { container } = render(<Entry />);

  const name = screen.getByText(appName, { exact: true });
  expect(name.textContent).toBe("Nerve Wiki");
  // Text, no heading: the page's h1 is its only heading.
  expect(screen.getAllByRole("heading").map((heading) => heading.textContent)).toEqual(["Sign in"]);
  expect(name.closest("h1, h2, h3, h4, h5, h6")).toBeNull();
  const mark = container.querySelector("svg");
  expect(mark?.getAttribute("aria-hidden")).toBe("true");
  expect(name.contains(mark)).toBe(true);
  // The page's content after it.
  expect(name.compareDocumentPosition(screen.getByRole("heading", { level: 1 }))).toBe(
    Node.DOCUMENT_POSITION_FOLLOWING
  );
  expect(name.compareDocumentPosition(screen.getByText("The form"))).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  expect(document.title).toBe(`Sign in · ${appName}`);
});
