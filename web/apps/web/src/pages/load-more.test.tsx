import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef } from "react";
import { unstable_serialize, type Cache, type ScopedMutator } from "swr";
import { expect, onTestFinished, test, vi } from "vitest";

import { LoadMoreButton, readNext, useLoadMore, type MoreRead } from "./load-more";

// A list's Load more, as its lists use it; their focus is in their own tests.

function List({ more }: { more: () => Promise<MoreRead> }) {
  const heading = useRef<HTMLHeadingElement>(null);
  const load = useLoadMore(more, () => null, heading);
  return (
    <>
      <h2 ref={heading} tabIndex={-1}>
        Items
      </h2>
      <LoadMoreButton more={load} label="More" />
    </>
  );
}

test("pressed again as it reads, it reads nothing more: a list's reads may queue, one more page each", async () => {
  const user = userEvent.setup();
  let answer: ((read: MoreRead) => void) | undefined;
  const more = vi.fn(() => new Promise<MoreRead>((resolve) => (answer = resolve)));
  render(<List more={more} />);
  const button = screen.getByRole("button", { name: "More" });

  await user.click(button);
  await user.click(button);
  expect(more).toHaveBeenCalledTimes(1);

  act(() => answer?.({ added: [], whole: false, last: undefined }));
  await waitFor(() => expect(button.getAttribute("aria-busy")).toBeNull());
  await user.click(button);
  expect(more).toHaveBeenCalledTimes(2);
});

// M8/P3B fix check 1, N2 (M8/P1 fix check 3, R3-1): SWR's mutate rejects once the login's cache is gone.
test("a page added to a list being read, which is read again, the login's cache gone meanwhile, leaves no rejection unhandled", async () => {
  const rejections: unknown[] = [];
  const record = (reason: unknown) => void rejections.push(reason);
  process.on("unhandledRejection", record);
  onTestFinished(() => {
    process.off("unhandledRejection", record);
  });
  type Page = { data: string[]; next_cursor: string | null };
  const key = ["list"];
  // The list read up to a, being read again.
  const cache = new Map([[unstable_serialize(key), { isValidating: true }]]) as unknown as Cache;
  const calls: string[] = [];
  const mutate = (async (_key: unknown, data?: (list: Page[]) => Page[]) => {
    if (data === undefined) {
      calls.push("read again");
      throw new TypeError("the login's cache is gone");
    }
    calls.push("add");
    return data([{ data: ["a"], next_cursor: "a" }]);
  }) as unknown as ScopedMutator;

  const read = await readNext(
    { cache, mutate },
    key,
    "a",
    async () => ({ data: ["b"], next_cursor: null }),
    (page: Page) => page.data
  );

  expect(read).toEqual({ added: ["b"], whole: true, last: "b", appended: true });
  expect(calls).toEqual(["add", "read again"]);
  await act(async () => new Promise((resolve) => setTimeout(resolve, 10)));
  expect(rejections).toEqual([]);
});
