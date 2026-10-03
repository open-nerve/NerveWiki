import { render } from "@testing-library/react";
import { expect, test } from "vitest";

import { I18nProvider } from "../i18n/i18n";
import { ConflictDiff } from "./conflict-view";

const lines = Array.from({ length: 12 }, (_, i) => `line ${i.toString()}`).join("\n");

/** The diff of mine, lines and a line added, against theirs, lines, in Chinese. */
const diff = (unfolded: boolean) => (
  <I18nProvider locale="zh-CN">
    <ConflictDiff theirs={`${lines}\n`} mine={`${lines}\nmine\n`} unfolded={unfolded} />
  </I18nProvider>
);

test("folds the unchanged stretches, in the app's language, unless unfolded", () => {
  const { container, rerender } = render(diff(false));

  expect(container.querySelector(".cm-collapsedLines")?.textContent).toMatch(/^\d+ 行未改动$/);
  rerender(diff(true));
  expect(container.querySelector(".cm-collapsedLines")).toBeNull();
  expect(container.textContent).toContain("line 3");
});
