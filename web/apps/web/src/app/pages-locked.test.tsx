import { render } from "@testing-library/react";
import { expect, test } from "vitest";

import { translator } from "../i18n/i18n";
import { ApiError } from "../services/api";
import { pagesLocked } from "./pages-locked";

const t = translator("en");

/** linking.pages_locked of the pages p2, which Bob edits, and p4, which u-ada does. */
const locked = new ApiError(409, {
  status: 409,
  code: "linking.pages_locked",
  title: "Conflict",
  locks: [
    { page_id: "p2", user_id: "u-bob", display_name: "Bob" },
    { page_id: "p4", user_id: "u-ada", display_name: "Ada" },
  ],
});
/** The tree's titles: p2 is Linux, p4 Notes. */
const titleOf = (id: string) => ({ p2: "Linux", p4: "Notes" })[id];

/** The text of each part of what node says: its paragraph, then each item. */
function parts(node: ReturnType<typeof pagesLocked>): string[] {
  const { container } = render(<div>{node}</div>);
  return [...container.querySelectorAll("p, li")].map((each) => each.textContent ?? "");
}

test("pagesLocked says the change writes the pages' links again, and names who edits each, the account itself too", () => {
  expect(parts(pagesLocked(locked, t, "u-ada", titleOf))).toEqual([
    "This change would write the links on these pages again, and they are being edited. Try again once their editors are done; a notebook admin can also release a lock.",
    "Bob is editing “Linux”.",
    "You are editing “Notes”.",
  ]);
  expect(parts(pagesLocked(locked, translator("zh-CN"), "u-ada", titleOf))).toEqual([
    "这次改动需要改写这些页里的链接，而它们正在被编辑。等编辑结束后再试；笔记本管理员也可以解除锁定。",
    "Bob 正在编辑「Linux」。",
    "你正在编辑「Notes」。",
  ]);
});

test("pagesLocked says nothing of a page the tree does not know, nor of another problem", () => {
  expect(pagesLocked(locked, t, "u-ada", (id) => (id === "p2" ? "Linux" : undefined))).toBeUndefined();
  expect(
    pagesLocked(
      new ApiError(409, { status: 409, code: "linking.pages_locked", title: "Conflict" }),
      t,
      "u-ada",
      titleOf
    )
  ).toBeUndefined();
  expect(
    pagesLocked(
      new ApiError(409, {
        status: 409,
        code: "page.locked",
        title: "Conflict",
        lock: { page_id: "p2", user_id: "u-bob", display_name: "Bob" },
      }),
      t,
      "u-ada",
      titleOf
    )
  ).toBeUndefined();
});
