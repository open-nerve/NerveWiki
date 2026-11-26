import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";

import { I18nProvider } from "../../i18n/i18n";
import { EditOutline, OutlineBox } from "./edit-outline";

// M13 closeout (review B's M13CB-F1-M1): a heading of the edit's outline, pressed, focuses the editor at its line. A
// key held on it pressed it once, and its repeats fell in the editor: Enter held on Tasks put six new lines before its
// heading, which the autosave saves. Pressed by the keyboard, the heading swallows the held key's repeats, as a dialog
// closing does (decision 967).
test("Enter held on a heading of the edit's outline leads to its line once, its repeats typing nothing in the editor; let go, Enter is the editor's", async () => {
  const user = userEvent.setup();
  const box = new OutlineBox();
  box.told(
    [
      { from: 0, level: 2, text: "Tasks" },
      { from: 20, level: 2, text: "Notes" },
    ],
    0
  );
  render(
    <I18nProvider locale="en">
      <EditOutline box={box} />
      <textarea aria-label="Source" />
    </I18nProvider>
  );
  const source = screen.getByRole<HTMLTextAreaElement>("textbox", { name: "Source" });
  const revealed = vi.fn();
  box.reveal = (index) => {
    revealed(index);
    source.focus();
  };
  screen.getByRole("button", { name: "Tasks" }).focus();

  await user.keyboard("{Enter>6}");
  expect(revealed).toHaveBeenCalledTimes(1);
  expect(revealed).toHaveBeenCalledWith(0);
  expect(document.activeElement).toBe(source);
  expect(source.value).toBe("");

  await user.keyboard("{/Enter}{Enter}");
  expect(source.value).toBe("\n");
  expect(revealed).toHaveBeenCalledTimes(1);
});
