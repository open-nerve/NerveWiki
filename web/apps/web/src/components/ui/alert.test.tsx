import { readFileSync } from "node:fs";
import { URL as FileURL } from "node:url";

import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";

import { Alert } from "./alert";

// The alert's look (M13/P1 design 4.2.3): red text on a red tint, a fainter red border.

const look = ["rounded-lg", "border", "border-destructive/35", "bg-destructive/5", "text-destructive"];

test("an alert reads red on a red tint", () => {
  render(<Alert>Not saved.</Alert>);
  const { classList } = screen.getByRole("alert");
  for (const each of look) expect(classList.contains(each), each).toBe(true);
});

test("the diff's failure, which copies the alert's look, keeps to it", () => {
  // The editor does not import the app's alert (text-diff.tsx's Failed): its copy is read as written.
  const source = readFileSync(new FileURL("../../editor/text-diff.tsx", import.meta.url), "utf8");
  const [, copied = ""] = /role="alert"\s+className="([^"]*)"/.exec(source) ?? [];
  const classes = copied.split(/\s+/);
  for (const each of look) expect(classes, each).toContain(each);
  expect(classes).not.toContain("border-destructive/50");
});
