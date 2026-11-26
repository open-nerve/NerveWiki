import { act, render, screen, waitFor } from "@testing-library/react";
import { observable, runInAction } from "mobx";
import type { ReactNode } from "react";
import { createMemoryRouter, RouterProvider, type InitialEntry } from "react-router";
import { expect, test, vi } from "vitest";

import { arrived } from "./arrival";
import { cardClass, SavedOutput, SettingsCard, SettingsPage, SettingsRow, useSettingsHeading } from "./settings-page";

// A page of the settings' mode (M13/P3/S1, decisions 815, 820): its name its h1, which takes the arrival focus.

/** Renders what at /, the router's first entry entry. */
function at(entry: InitialEntry, what: ReactNode) {
  const router = createMemoryRouter([{ path: "/", element: what }], { initialEntries: [entry] });
  return render(<RouterProvider router={router} />);
}

test("arrived at, a settings page's h1 takes the focus: its name, a heading the program focuses", async () => {
  at({ pathname: "/", state: arrived }, <SettingsPage title="Profile">The page</SettingsPage>);

  const title = screen.getByRole("heading", { level: 1, name: "Profile" });
  await waitFor(() => expect(document.activeElement).toBe(title));
  expect(title.getAttribute("tabindex")).toBe("-1");
  expect(screen.getByText("The page")).toBeTruthy();
});

test("opened, not arrived at, a settings page's h1 takes no focus", async () => {
  at("/", <SettingsPage title="Profile">The page</SettingsPage>);

  const title = screen.getByRole("heading", { level: 1, name: "Profile" });
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(document.activeElement).not.toBe(title);
});

/** Focus gives the focus to the settings page's h1, as a list's removal does. */
function Focus() {
  const heading = useSettingsHeading();
  return (
    <button type="button" onClick={() => heading.current?.focus()}>
      Focus
    </button>
  );
}

test("useSettingsHeading is the page's h1; outside a settings page it throws", async () => {
  at(
    "/",
    <SettingsPage title="Access tokens">
      <Focus />
    </SettingsPage>
  );
  act(() => screen.getByRole("button", { name: "Focus" }).click());
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Access tokens" }));

  vi.spyOn(console, "error").mockImplementation(() => {});
  expect(() => render(<Focus />)).toThrow("useSettingsHeading is used outside SettingsPage");
});

test("SavedOutput says Saved. or nothing, marked for its check; its round turns each time it is written, not as it empties or stays (decision 841)", () => {
  const { rerender } = render(<SavedOutput saved={false} label="Saved." />);
  const output = screen.getByRole("status");
  expect(output.textContent).toBe("");
  expect(output.classList.contains("nw-saved")).toBe(true);

  const rounds: (string | null)[] = [];
  for (const saved of [false, true, false, true, false, false, true, true]) {
    rerender(<SavedOutput saved={saved} label="Saved." />);
    expect(output.textContent).toBe(saved ? "Saved." : "");
    rounds.push(output.getAttribute("data-round"));
  }
  expect(rounds).toEqual(["0", "1", "1", "0", "0", "0", "1", "1"]);
});

test("a row's controls follow the store they read: SettingsRow draws them in its own render, which it observes (decision 849)", () => {
  const theme = observable({ dark: false });
  render(
    <SettingsRow title="Theme">
      {({ title }) => <output aria-labelledby={title}>{theme.dark ? "Dark" : "Light"}</output>}
    </SettingsRow>
  );
  expect(screen.getByRole("status", { name: "Theme" }).textContent).toBe("Light");

  act(() => runInAction(() => (theme.dark = true)));

  expect(screen.getByRole("status", { name: "Theme" }).textContent).toBe("Dark");
});

test("a card is cardClass, a list of them too: the one card the lists' pages and the homes share (M13/P5/S1, decision 925)", () => {
  render(
    <>
      <SettingsCard list label="Tokens">
        <li>deploy</li>
      </SettingsCard>
      <SettingsCard>
        <p>None</p>
      </SettingsCard>
    </>
  );

  expect(screen.getByRole("list", { name: "Tokens" }).className).toBe(cardClass);
  expect(screen.getByText("None").parentElement?.className).toBe(cardClass);
  expect(cardClass).toBe("divide-y divide-border rounded-card border border-border bg-surface");
});
