import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { expect, test } from "vitest";

import { I18nProvider } from "../i18n/i18n";
import { ConfirmDialog } from "./confirm-dialog";

/** A caller that holds the dialog and stays: it opens it with its own button and logs each closing. */
function Holder({ closed, tone }: { closed: boolean[]; tone?: "destructive" | "default" }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Open
      </button>
      <ConfirmDialog
        held={{ open, onOpenChange: setOpen, onClosed: (done) => closed.push(done) }}
        title="Delete it?"
        description="It goes."
        confirmLabel="Delete"
        sendingLabel="Deleting…"
        cancelLabel="Cancel"
        confirm={() => Promise.resolve()}
        tone={tone}
      />
    </>
  );
}

test("a held dialog tells its caller whether confirm went through, and opens anew after it did", async () => {
  const user = userEvent.setup();
  const closed: boolean[] = [];
  render(
    <I18nProvider locale="en">
      <Holder closed={closed} />
    </I18nProvider>
  );

  await user.click(screen.getByRole("button", { name: "Open" }));
  await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete" }));
  await waitFor(() => expect(closed).toEqual([true]));
  expect(screen.queryByRole("alertdialog")).toBeNull();

  await user.click(screen.getByRole("button", { name: "Open" }));
  const again = screen.getByRole("alertdialog");
  expect((within(again).getByRole("button", { name: "Delete" }) as HTMLButtonElement).disabled).toBe(false);
  await user.click(within(again).getByRole("button", { name: "Cancel" }));

  await waitFor(() => expect(closed).toEqual([true, false]));
});

test("a dialog by its trigger, the trigger staying, opens anew after confirm went through", async () => {
  const user = userEvent.setup();
  let confirmed = 0;
  render(
    <I18nProvider locale="en">
      <ConfirmDialog
        trigger={<button type="button">Export</button>}
        focusAfter={() => {}}
        title="Export it?"
        description="It goes in a zip."
        confirmLabel="Start"
        sendingLabel="Starting…"
        cancelLabel="Cancel"
        confirm={async () => {
          confirmed += 1;
        }}
        tone="default"
      />
    </I18nProvider>
  );

  for (const time of [1, 2]) {
    // oxlint-disable-next-line no-await-in-loop -- one confirm after the other
    await user.click(screen.getByRole("button", { name: "Export" }));
    // oxlint-disable-next-line no-await-in-loop -- its dialog
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Start" }));
    // oxlint-disable-next-line no-await-in-loop -- closed
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(confirmed).toBe(time);
  }
});

test("the confirm button is destructive unless the dialog's tone is the default one", async () => {
  const user = userEvent.setup();
  for (const [tone, look] of [
    [undefined, "text-destructive"],
    ["default", "bg-primary"],
  ] as const) {
    const { unmount } = render(
      <I18nProvider locale="en">
        <Holder closed={[]} tone={tone} />
      </I18nProvider>
    );
    // oxlint-disable-next-line no-await-in-loop -- one tone after another
    await user.click(screen.getByRole("button", { name: "Open" }));
    const button = within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete" });
    expect(button.classList.contains(look), String(tone)).toBe(true);
    // Its buttons a line each, to the right, where one line holds not both: at 320, English (decision 958, extended).
    expect(button.parentElement?.className).toBe("flex flex-wrap justify-end gap-2");
    unmount();
  }
});

test("the dialog says more below its description, and a refusal its explain makes a list of", async () => {
  const user = userEvent.setup();
  render(
    <I18nProvider locale="en">
      <ConfirmDialog
        trigger={<button type="button">Open</button>}
        title="Start it?"
        description="It starts."
        more={<p>Its page raw holds 3 nodes.</p>}
        confirmLabel="Start"
        sendingLabel="Starting…"
        cancelLabel="Cancel"
        confirm={() => Promise.reject(new Error("refused"))}
        explain={() => (
          <ul>
            <li>Notes</li>
            <li>Guide</li>
          </ul>
        )}
        tone="default"
      />
    </I18nProvider>
  );

  await user.click(screen.getByRole("button", { name: "Open" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Start it?" });
  expect(within(dialog).getByText("Its page raw holds 3 nodes.")).toBeTruthy();
  await user.click(within(dialog).getByRole("button", { name: "Start" }));

  const alert = await within(dialog).findByRole("alert");
  expect(
    within(alert)
      .getAllByRole("listitem")
      .map((item) => item.textContent)
  ).toEqual(["Notes", "Guide"]);
});

/** A dialog by its trigger that says more, if any, below its description. */
const saying = (more?: ReactNode) => (
  <I18nProvider locale="en">
    <ConfirmDialog
      trigger={<button type="button">Open</button>}
      title="Start it?"
      description="It starts."
      more={more}
      confirmLabel="Start"
      sendingLabel="Starting…"
      cancelLabel="Cancel"
      confirm={() => Promise.resolve()}
      tone="default"
    />
  </I18nProvider>
);

test("what more the dialog says describes it with its description; without more, the description alone does", async () => {
  const user = userEvent.setup();
  const { rerender } = render(
    saying(
      <>
        <p>Its page raw holds 3 nodes.</p>
        <p>Links may lead nowhere.</p>
      </>
    )
  );

  await user.click(screen.getByRole("button", { name: "Open" }));
  expect(
    await screen.findByRole("alertdialog", {
      name: "Start it?",
      description: "It starts. Its page raw holds 3 nodes. Links may lead nowhere.",
    })
  ).toBeTruthy();

  rerender(saying());
  expect(screen.getByRole("alertdialog", { name: "Start it?", description: "It starts." })).toBeTruthy();
});

/** held is a confirm the test settles: each call waits in calls until it is resolved or rejected. */
function held() {
  const calls: { resolve: () => void; reject: (error: unknown) => void }[] = [];
  const confirm = () =>
    new Promise<void>((resolve, reject) => {
      calls.push({ resolve, reject });
    });
  return { calls, confirm };
}

/** deleting is a dialog by its trigger whose confirm is confirm, asking for a typed word when typed. */
const deleting = (confirm: () => Promise<void>, typed = false) => (
  <I18nProvider locale="en">
    <ConfirmDialog
      trigger={<button type="button">Open</button>}
      title="Delete it?"
      description="It goes."
      confirmLabel="Delete"
      sendingLabel="Deleting…"
      cancelLabel="Cancel"
      confirm={confirm}
      typedConfirmation={typed ? { label: "Name of the notebook", value: "plans" } : undefined}
    />
  </I18nProvider>
);

/** opened is the dialog of deleting opened, by a click on its trigger. */
async function opened(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Open" }));
  return screen.findByRole("alertdialog", { name: "Delete it?" });
}

// M12/P2/S1, 23.16: a disabled button drops the focus (Chromium gives it to the body at its next rendering), so the
// buttons are aria-disabled while confirm is out (v0.1 design 13.2, item 12).
test("while confirm is out, its buttons are aria-disabled, not disabled, the focus staying on confirm; pressed again, Esc and Cancel do nothing, and the dialog stays", async () => {
  const user = userEvent.setup();
  const { calls, confirm } = held();
  render(deleting(confirm));
  const dialog = await opened(user);

  await user.click(within(dialog).getByRole("button", { name: "Delete" }));
  const sending = within(dialog).getByRole("button", { name: "Deleting…" });
  const cancel = within(dialog).getByRole("button", { name: "Cancel" });
  expect([
    sending.getAttribute("aria-disabled"),
    sending.hasAttribute("disabled"),
    sending.getAttribute("aria-busy"),
    cancel.getAttribute("aria-disabled"),
    cancel.hasAttribute("disabled"),
  ]).toEqual(["true", false, "true", "true", false]);
  expect(document.activeElement).toBe(sending);

  await user.click(sending);
  await user.keyboard("{Escape}");
  await user.click(cancel);
  expect(screen.getByRole("alertdialog", { name: "Delete it?" })).toBe(dialog);
  expect(calls).toHaveLength(1);
});

test("a refusal stays in the dialog, the focus on confirm, which sends again", async () => {
  const user = userEvent.setup();
  const { calls, confirm } = held();
  render(deleting(confirm));
  const dialog = await opened(user);
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));
  await waitFor(() => expect(calls).toHaveLength(1));

  await act(async () => calls[0]?.reject(new Error("refused")));
  expect((await within(dialog).findByRole("alert")).textContent).toBe("Something went wrong (Error).");
  const again = within(dialog).getByRole("button", { name: "Delete" });
  expect(document.activeElement).toBe(again);
  expect(again.hasAttribute("aria-disabled")).toBe(false);

  await user.click(again);
  expect(calls).toHaveLength(2);
});

test("a refusal that finds the focus lost gives it back to confirm", async () => {
  const user = userEvent.setup();
  const { calls, confirm } = held();
  render(deleting(confirm));
  const dialog = await opened(user);
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));
  await waitFor(() => expect(calls).toHaveLength(1));
  // Chromium's focus fixup, which jsdom does not do: the focus falls to the body.
  act(() => (document.activeElement as HTMLElement).blur());
  expect(document.activeElement).toBe(document.body);

  await act(async () => calls[0]?.reject(new Error("refused")));
  await within(dialog).findByRole("alert");
  await waitFor(() => expect(document.activeElement).toBe(within(dialog).getByRole("button", { name: "Delete" })));
});

test("Enter in the typed word sends once, however often it is pressed; refused, the focus stays in the field", async () => {
  const user = userEvent.setup();
  const { calls, confirm } = held();
  render(deleting(confirm, true));
  const dialog = await opened(user);
  const field = within(dialog).getByRole("textbox", { name: "Name of the notebook" });
  await user.type(field, "plans");

  // Twice before the dialog renders again, as a key held down repeats it: what is out is not state yet.
  act(() => {
    for (const _ of [1, 2]) {
      field.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true }));
    }
  });
  await user.keyboard("{Enter}");
  expect(calls).toHaveLength(1);

  await act(async () => calls[0]?.reject(new Error("refused")));
  await within(dialog).findByRole("alert");
  expect(document.activeElement).toBe(field);
});
