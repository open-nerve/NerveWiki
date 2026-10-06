import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
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

test("the confirm button is destructive unless the dialog's tone is the default one", async () => {
  const user = userEvent.setup();
  for (const [tone, look] of [
    [undefined, "bg-destructive"],
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
    unmount();
  }
});
