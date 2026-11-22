import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "./dropdown-menu";

// The menus' looks (M13/P1 design 4.2.6, 4.2.9). jsdom runs no animation, and Radix's Presence reads no animation's
// name in it: what a menu shows as it opens and closes is in its classes here, and in a browser in L5.

/** opened renders a menu of an ordinary item, a destructive one and a choice, and opens it. */
async function opened() {
  const user = userEvent.setup();
  render(
    <DropdownMenu>
      <DropdownMenuTrigger>Actions</DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem>Rename</DropdownMenuItem>
        <DropdownMenuItem tone="destructive">Delete</DropdownMenuItem>
        <DropdownMenuRadioGroup value="light">
          <DropdownMenuRadioItem value="light">Light</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
  await user.click(screen.getByRole("button", { name: "Actions" }));
  return await screen.findByRole("menu");
}

test("a destructive item reads red, an ordinary one does not", async () => {
  await opened();
  const remove = screen.getByRole("menuitem", { name: "Delete" });
  for (const look of ["text-destructive", "focus:text-destructive", "[&>svg]:text-destructive"]) {
    expect(remove.classList.contains(look), look).toBe(true);
  }
  // The red is no lighter for the highlight's text colour, which comes after it otherwise.
  expect(remove.classList.contains("focus:text-foreground")).toBe(false);
  // Its tone is the component's: no attribute of the DOM.
  expect(remove.getAttribute("tone")).toBeNull();
  const rename = screen.getByRole("menuitem", { name: "Rename" });
  expect([...rename.classList].filter((name) => name.includes("destructive"))).toEqual([]);
});

test("the menu animates in, never out", async () => {
  const menu = await opened();
  expect(menu.getAttribute("data-state")).toBe("open");
  expect(menu.classList.contains("data-[state=open]:animate-pop-in")).toBe(true);
  // An exit's animation would keep a closed menu mounted until it ends: two menus, as another opens (00-M13 737).
  expect(menu.className).not.toContain("data-[state=closed]:");
});

test("the item's ring is the keyboard's", async () => {
  await opened();
  // The pointer's highlight is its background alone; the keyboard's has the ring too (decision 760).
  for (const item of [
    screen.getByRole("menuitem", { name: "Rename" }),
    screen.getByRole("menuitem", { name: "Delete" }),
    screen.getByRole("menuitemradio", { name: "Light" }),
  ]) {
    const name = item.textContent ?? "";
    // Its one ring width, the keyboard's: focus:ring-2, focus-visible:ring-2 (which Radix's pointer matches too) or
    // data-highlighted:ring-2 would ring the pointer's highlight.
    expect(
      [...item.classList].filter((each) => /(?:^|:)ring-(?:\d|\[)/.test(each)),
      name
    ).toEqual(["in-data-[modality=keyboard]:focus:ring-2"]);
    // Under forced colours, which drop the ring, a transparent outline shows where the focus is (decision 768).
    expect(item.classList.contains("focus-visible:outline-hidden"), name).toBe(true);
  }
  // An item's own icon is quieter than its text; a destructive item's is red.
  expect(screen.getByRole("menuitem", { name: "Rename" }).classList.contains("[&>svg]:text-muted-foreground")).toBe(
    true
  );
  // The choice's check is sized: lucide's would be 24px, where the item's direct icons alone are sized.
  const check = screen.getByRole("menuitemradio", { name: "Light" }).querySelector("svg");
  expect(check?.classList.contains("size-3.5")).toBe(true);
});

test("a submenu opens from its item, by the keyboard too, and animates in, never out", async () => {
  const user = userEvent.setup();
  render(
    <DropdownMenu>
      <DropdownMenuTrigger>Account</DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem>Settings</DropdownMenuItem>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>Appearance</DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuItem>Light</DropdownMenuItem>
            <DropdownMenuItem>Dark</DropdownMenuItem>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
      </DropdownMenuContent>
    </DropdownMenu>
  );
  const trigger = screen.getByRole("button", { name: "Account" });

  // By the pointer: a click opens it (M13/P2/S1; the focus stays on its item, review A-N1).
  await user.click(trigger);
  const appearance = await screen.findByRole("menuitem", { name: "Appearance" });
  expect(appearance.getAttribute("aria-haspopup")).toBe("menu");
  await user.click(appearance);
  expect(await screen.findByRole("menuitem", { name: "Light" })).toBeTruthy();
  expect(document.activeElement).toBe(appearance);
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryAllByRole("menu")).toEqual([]));

  // By the keyboard: Right opens it at its first item; Escape closes both.
  trigger.focus();
  await user.keyboard("{Enter}");
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Settings" })));
  await user.keyboard("{ArrowDown}");
  expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Appearance" }));
  await user.keyboard("{ArrowRight}");
  const light = await screen.findByRole("menuitem", { name: "Light" });
  await waitFor(() => expect(document.activeElement).toBe(light));
  const sub = screen.getAllByRole("menu").find((menu) => menu.contains(light));
  expect(sub?.getAttribute("data-state")).toBe("open");
  expect(sub?.classList.contains("data-[state=open]:animate-pop-in")).toBe(true);
  // An exit's animation would keep it mounted after Escape (00-M13 737).
  expect(sub?.className).not.toContain("data-[state=closed]:");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryAllByRole("menu")).toEqual([]));
  await waitFor(() => expect(document.activeElement).toBe(trigger));
});
