import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, test } from "vitest";

import { FormField } from "../components/form-field";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { I18nProvider } from "../i18n/i18n";
import { ApiError } from "../services/api";
import { useForm } from "./form";

// M12/P2/S1, 9.1 and 38.3: a refusal only the banner above the form says, whose button was disabled while the form
// was out, leaves the focus nowhere (Chromium gives a disabled button's focus to the body); the banner takes it then.
// A focus not lost stays: the undo dialog's aria-disabled Undo (M8), or where the reader put it (v0.1 design 13.2,
// items 12 and 26).

/** held is a send the test settles: each call waits in calls until it is resolved or rejected. */
function held() {
  const calls: { resolve: () => void; reject: (error: unknown) => void }[] = [];
  const send = () =>
    new Promise<void>((resolve, reject) => {
      calls.push({ resolve, reject });
    });
  return { calls, send };
}

/** Named is a form of one field, Name, whose button is disabled while it is out, or aria-disabled as the undo dialog's. */
function Named({ send, ariaDisabled = false }: { send: () => Promise<void>; ariaDisabled?: boolean }) {
  const { ref, sending, banner, problemOf, submit } = useForm(["name"] as const);
  const [name, setName] = useState("Notes");
  return (
    <>
      <input aria-label="Elsewhere" />
      <form
        ref={ref}
        noValidate
        onSubmit={(event) => {
          event.preventDefault();
          void submit({}, send);
        }}
      >
        {banner !== undefined && <Alert>{banner}</Alert>}
        <FormField
          label="Name"
          name="name"
          value={name}
          error={problemOf("name")}
          onChange={(event) => setName(event.target.value)}
        />
        {ariaDisabled ? (
          <Button type="submit" aria-disabled={sending || undefined}>
            Save
          </Button>
        ) : (
          <Button type="submit" disabled={sending}>
            Save
          </Button>
        )}
      </form>
    </>
  );
}

const cycle = () => new ApiError(409, { status: 409, code: "page.cycle", title: "Conflict" });
const cycleText = "A page cannot move under itself or one of its subpages.";

/** sent is the form rendered with a held send, Save clicked: the send is out. */
async function sent(ariaDisabled = false) {
  const user = userEvent.setup();
  const { calls, send } = held();
  render(
    <I18nProvider locale="en">
      <Named send={send} ariaDisabled={ariaDisabled} />
    </I18nProvider>
  );
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(calls).toHaveLength(1));
  return { user, calls, save: screen.getByRole("button", { name: "Save" }) };
}

test("a refusal only the banner says gives it the focus its disabled button dropped", async () => {
  const { calls, save } = await sent();
  expect(document.activeElement).toBe(save);

  await act(async () => calls[0]?.reject(cycle()));
  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toBe(cycleText);
  await waitFor(() => expect(document.activeElement).toBe(alert));
  expect(alert.tabIndex).toBe(-1);
});

test("a refusal only the banner says gives it the focus fallen to the body, as Chromium's fixup leaves it", async () => {
  const { calls } = await sent();
  // Chromium's focus fixup of the disabled button, which jsdom does not do (nor blur a disabled element).
  act(() => {
    const elsewhere = screen.getByRole("textbox", { name: "Elsewhere" });
    elsewhere.focus();
    elsewhere.blur();
  });
  expect(document.activeElement).toBe(document.body);

  await act(async () => calls[0]?.reject(cycle()));
  const alert = await screen.findByRole("alert");
  await waitFor(() => expect(document.activeElement).toBe(alert));
});

test("a refusal the reader moved the focus away from meanwhile leaves it where they put it", async () => {
  const { user, calls } = await sent();
  const elsewhere = screen.getByRole("textbox", { name: "Elsewhere" });
  await user.click(elsewhere);

  await act(async () => calls[0]?.reject(cycle()));
  expect((await screen.findByRole("alert")).textContent).toBe(cycleText);
  expect(document.activeElement).toBe(elsewhere);
});

test("a refusal a field says gives the field the focus, not the banner", async () => {
  const { calls } = await sent();

  await act(async () =>
    calls[0]?.reject(
      new ApiError(422, {
        status: 422,
        code: "validation_failed",
        title: "Unprocessable Entity",
        errors: [{ field: "name", code: "too_long", message: "too long" }],
      })
    )
  );
  const field = screen.getByRole("textbox", { name: "Name" });
  await waitFor(() => expect(document.activeElement).toBe(field));
  expect(field.getAttribute("aria-invalid")).toBe("true");
  expect(screen.queryByRole("alert")).toBeNull();
});

test("a refusal, its button aria-disabled and focused, leaves the focus on it", async () => {
  const { calls, save } = await sent(true);
  expect(save.getAttribute("aria-disabled")).toBe("true");
  expect(document.activeElement).toBe(save);

  await act(async () => calls[0]?.reject(cycle()));
  expect((await screen.findByRole("alert")).textContent).toBe(cycleText);
  expect(document.activeElement).toBe(save);
});
