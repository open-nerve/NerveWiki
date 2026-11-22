import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { expect, onTestFinished, test, vi } from "vitest";

import { NativeSelect, useChoice } from "./native-select";

// A choice whose use reads something, on a native select (M5-polish 26.5). jsdom sends no change for a key: the tests
// send the key, then the change, as the browsers do; a change with no key before it is the pointer's or the open
// list's. jsdom's platform is not macOS.

/** Choice is a select of a, b and c, a applied first; each choice used is logged. */
function Choice({ used }: { used: string[] }) {
  const [applied, setApplied] = useState("a");
  const choice = useChoice(applied, (value) => {
    used.push(value);
    setApplied(value);
  });
  return (
    <NativeSelect aria-label="Choice" {...choice}>
      <option value="a">A</option>
      <option value="b">B</option>
      <option value="c">C</option>
    </NativeSelect>
  );
}

/** shown renders Choice and answers its select and the choices used. */
function shown() {
  const used: string[] = [];
  render(<Choice used={used} />);
  return { select: screen.getByRole("combobox", { name: "Choice" }) as HTMLSelectElement, used };
}

test("a step of the keys waits, shown, until Enter, which uses it and keeps the browser from opening the list; Enter with nothing waiting is the browser's", () => {
  const { select, used } = shown();

  fireEvent.keyDown(select, { key: "ArrowDown" });
  fireEvent.change(select, { target: { value: "b" } });
  expect(select.value).toBe("b");
  expect(used).toEqual([]);

  // Chromium opens the list on Enter (M12/P2/S6 review B, M1): not once Enter used what waited.
  expect(fireEvent.keyDown(select, { key: "Enter" })).toBe(false);
  expect(used).toEqual(["b"]);
  expect(fireEvent.keyDown(select, { key: "Enter" })).toBe(true);
  expect(used).toEqual(["b"]);
});

test("the keys that open the list are no step, and each key says anew whether it steps: a change after them is used at once", () => {
  const { select, used } = shown();

  // An arrow that changed nothing at the first option, then Alt and an arrow, which opens the list.
  fireEvent.keyDown(select, { key: "ArrowUp" });
  fireEvent.keyDown(select, { key: "ArrowDown", altKey: true });
  fireEvent.change(select, { target: { value: "c" } });
  expect(used).toEqual(["c"]);

  fireEvent.keyDown(select, { key: " " });
  fireEvent.change(select, { target: { value: "b" } });
  expect(used).toEqual(["c", "b"]);

  fireEvent.keyDown(select, { key: "F4" });
  fireEvent.change(select, { target: { value: "a" } });
  expect(used).toEqual(["c", "b", "a"]);
});

test("stepped away from the choice in use and back to it, left, the select uses nothing: what is applied reads nothing again", () => {
  const { select, used } = shown();

  fireEvent.keyDown(select, { key: "ArrowDown" });
  fireEvent.change(select, { target: { value: "b" } });
  fireEvent.keyDown(select, { key: "ArrowUp" });
  fireEvent.change(select, { target: { value: "a" } });
  fireEvent.blur(select);

  expect(select.value).toBe("a");
  expect(used).toEqual([]);
});

test("on macOS the arrows and the others that step elsewhere are no step, as they open the list, whose choice is used at once; a character typed to find an option still waits", () => {
  const platform = vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
  onTestFinished(() => platform.mockRestore());
  const { select, used } = shown();

  for (const [key, value] of [
    ["ArrowDown", "b"],
    ["ArrowUp", "a"],
    ["End", "c"],
    ["Home", "a"],
    ["PageDown", "c"],
    ["PageUp", "a"],
  ] as const) {
    fireEvent.keyDown(select, { key });
    fireEvent.change(select, { target: { value } });
  }
  expect(used).toEqual(["b", "a", "c", "a", "c", "a"]);

  fireEvent.keyDown(select, { key: "b" });
  fireEvent.change(select, { target: { value: "b" } });
  expect(used).toEqual(["b", "a", "c", "a", "c", "a"]);
  fireEvent.blur(select);
  expect(used).toEqual(["b", "a", "c", "a", "c", "a", "b"]);
});

test("it is drawn by the app, its paint contained", () => {
  render(
    <NativeSelect aria-label="Where">
      <option>Everywhere</option>
    </NativeSelect>
  );
  const { classList } = screen.getByRole("combobox", { name: "Where" });
  // The app's chevron, appearance none, which forced colours give back to the browser (styles.css's nw-select).
  expect(classList.contains("nw-select")).toBe(true);
  // WebKit counts the text of the option shown in the page's width otherwise (M5-polish 33.1; v0.1 design 13.2 item 31).
  expect(classList.contains("contain-paint")).toBe(true);
});

test("what is invalid shows on the select", () => {
  render(
    <NativeSelect aria-label="Expires" aria-invalid>
      <option>Never</option>
    </NativeSelect>
  );
  const select = screen.getByRole("combobox", { name: "Expires" });
  expect(select.getAttribute("aria-invalid")).toBe("true");
  expect(select.classList.contains("aria-invalid:border-destructive")).toBe(true);
  expect(select.classList.contains("aria-invalid:ring-destructive")).toBe(true);
});
