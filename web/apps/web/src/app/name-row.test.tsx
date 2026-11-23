import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import type { User } from "../services/account.service";
import { json, problem, signedInApp, userJSON } from "../test/fakes";
import { readAgain } from "../test/page-panel";
import { renderApp } from "../test/render";

// NameRow, a name changed in a settings card's row (M13/P3/S1, decision 822), as the profile's display name has it:
// Save once the field holds another name, gone once it is saved, the field then taking the focus.

/**
 * The profile page over a fake server that keeps the account: each change it was sent is in sent; held, the answers
 * wait for release; refuse answers in its stead when it returns an answer.
 */
function profile(refuse?: () => Response | Promise<Response> | undefined) {
  const sent: unknown[] = [];
  let me: User = userJSON;
  let gate: Promise<void> | undefined;
  let open: (() => void) | undefined;
  const app = signedInApp({
    "GET /api/v0/me": () => json(me),
    "PATCH /api/v0/me": async (request) => {
      const changes = (await request.clone().json()) as Partial<User>;
      sent.push(changes);
      await gate;
      const refused = refuse?.();
      if (refused !== undefined) {
        return refused;
      }
      me = { ...me, ...changes };
      return json(me);
    },
  });
  renderApp("/settings/profile", app);
  return {
    sent,
    hold() {
      gate = new Promise((resolve) => {
        open = resolve;
      });
    },
    release() {
      open?.();
      gate = undefined;
    },
    /** The name changed elsewhere: the next read has it. */
    rename(name: string) {
      me = { ...me, display_name: name };
    },
  };
}

const field = () => screen.findByLabelText<HTMLInputElement>("Display name");
const save = () => screen.queryByRole("button", { name: "Save" });
const said = () => screen.getByRole("status").textContent;
/** Lets what a key or a click sent reach the fake server. */
const settle = () => act(() => new Promise((resolve) => setTimeout(resolve, 20)));

test("until the name is edited there is no Save, and Enter sends nothing", async () => {
  const user = userEvent.setup();
  const server = profile();
  const name = await field();
  expect(name.value).toBe("Ada");
  expect(save()).toBeNull();

  await user.click(name);
  await user.keyboard("{Enter}");
  await settle();
  expect(server.sent).toEqual([]);
  expect(said()).toBe("");
});

test("Save shows once the field holds another name and goes as it is typed back; its place after the field stays", async () => {
  const user = userEvent.setup();
  profile();
  const name = await field();
  // The place of Save, there without it (review B's M6): the field does not move as it comes and goes.
  const slot = name.nextElementSibling;
  expect(slot).not.toBeNull();
  expect(slot?.childElementCount).toBe(0);

  await user.type(name, "!");
  const button = save();
  expect(button).not.toBeNull();
  expect(name.nextElementSibling).toBe(slot);
  expect(slot?.contains(button)).toBe(true);

  await user.keyboard("{Backspace}");
  expect(save()).toBeNull();
  expect(name.nextElementSibling).toBe(slot);
});

test("Save sends the name without its blanks; out, Save says so and sends nothing more; answered, it goes, the field takes the focus and Saved. shows", async () => {
  const user = userEvent.setup();
  const server = profile();
  const name = await field();
  server.hold();
  await user.clear(name);
  await user.type(name, "  Ada Lovelace ");
  const button = save();
  if (button === null) {
    throw new Error("no Save");
  }
  await user.click(button);
  await waitFor(() => expect(server.sent).toEqual([{ display_name: "Ada Lovelace" }]));

  // Not disabled: a disabled button would drop the focus (v0.1 design 13.2 item 12).
  expect(button.getAttribute("aria-disabled")).toBe("true");
  expect(button.getAttribute("aria-busy")).toBe("true");
  expect(button.hasAttribute("disabled")).toBe(false);
  act(() => name.focus());
  await user.keyboard("{Enter}");
  await user.click(button);
  await settle();
  expect(server.sent).toHaveLength(1);
  expect(document.activeElement).toBe(button);

  server.release();
  await waitFor(() => expect(said()).toBe("Saved."));
  expect(save()).toBeNull();
  expect(document.activeElement).toBe(name);
  expect(name.value).toBe("Ada Lovelace");
  expect(screen.getByRole("button", { name: "Ada Lovelace" })).toBeTruthy();
});

test("a name edited while its save is out keeps the edit and is not marked saved; Save sends it next", async () => {
  const user = userEvent.setup();
  const server = profile();
  const name = await field();
  server.hold();
  await user.clear(name);
  await user.type(name, "First name");
  await user.click(save() as HTMLElement);
  await waitFor(() => expect(server.sent).toHaveLength(1));
  await user.clear(name);
  await user.type(name, "Second name");
  server.release();
  await waitFor(() => expect(save()?.hasAttribute("aria-busy")).toBe(false));

  expect(said()).toBe("");
  expect(name.value).toBe("Second name");
  await user.click(save() as HTMLElement);
  await waitFor(() => expect(said()).toBe("Saved."));
  expect(server.sent).toEqual([{ display_name: "First name" }, { display_name: "Second name" }]);
});

test("a name the server refuses is marked under the field, which it alone describes and which takes the focus; saved right, the description describes the field again", async () => {
  const user = userEvent.setup();
  let refusing = true;
  profile(() =>
    refusing ? problem(422, "validation_failed", { errors: [{ field: "display_name", code: "too_long" }] }) : undefined
  );
  const name = await field();
  const description = screen.getByText("The name others see.");
  expect(name.getAttribute("aria-describedby")).toBe(description.id);

  await user.clear(name);
  await user.type(name, "A".repeat(101));
  await user.click(save() as HTMLElement);
  const note = await screen.findByText("At most 100 characters.");
  expect(name.getAttribute("aria-describedby")).toBe(note.id);
  expect(name.getAttribute("aria-invalid")).toBe("true");
  await waitFor(() => expect(document.activeElement).toBe(name));
  expect(said()).toBe("");

  refusing = false;
  await user.clear(name);
  await user.type(name, "Ada L");
  await user.click(save() as HTMLElement);
  await waitFor(() => expect(said()).toBe("Saved."));
  expect(screen.queryByText("At most 100 characters.")).toBeNull();
  expect(name.hasAttribute("aria-invalid")).toBe(false);
  expect(name.getAttribute("aria-describedby")).toBe(description.id);
});

test("with the name typed back as it was, its problem goes: the field neither invalid nor described by it, no Save, nothing sent (review A's M1)", async () => {
  const user = userEvent.setup();
  const server = profile();
  const name = await field();

  await user.clear(name);
  await user.keyboard("{Enter}");
  const note = await screen.findByText("Required.");
  expect(name.getAttribute("aria-describedby")).toBe(note.id);
  expect(name.getAttribute("aria-invalid")).toBe("true");

  await user.type(name, "Ada");
  expect(screen.queryByText("Required.")).toBeNull();
  expect(name.hasAttribute("aria-invalid")).toBe(false);
  expect(name.getAttribute("aria-describedby")).toBe(screen.getByText("The name others see.").id);
  expect(save()).toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
  await settle();
  expect(server.sent).toEqual([]);
});

test("a refusal of another kind shows under the row, the focus staying on Save; with the name typed back as it was, it goes", async () => {
  const user = userEvent.setup();
  profile(() => problem(403, "forbidden"));
  const name = await field();

  await user.type(name, "!");
  const button = save() as HTMLElement;
  await user.click(button);
  expect(await screen.findByRole("alert")).toBeTruthy();
  expect(document.activeElement).toBe(button);
  expect(said()).toBe("");

  await user.click(name);
  await user.keyboard("{Backspace}");
  expect(screen.queryByRole("alert")).toBeNull();
  expect(save()).toBeNull();
});

test("a name other only in its blanks is saved without a request, the field showing the name as it is", async () => {
  const user = userEvent.setup();
  const server = profile();
  const name = await field();

  await user.type(name, " ");
  await user.click(save() as HTMLElement);
  await waitFor(() => expect(said()).toBe("Saved."));
  expect(name.value).toBe("Ada");
  expect(save()).toBeNull();
  expect(server.sent).toEqual([]);
});

test("not edited, the field follows the name changed elsewhere", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  onTestFinished(() => void vi.useRealTimers());
  const server = profile();
  const name = await field();

  server.rename("Ada Lovelace");
  await readAgain();
  await waitFor(() => expect(name.value).toBe("Ada Lovelace"));
  expect(save()).toBeNull();
});
