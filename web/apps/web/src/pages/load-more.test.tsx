import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef } from "react";
import { expect, test, vi } from "vitest";

import { LoadMoreButton, useLoadMore, type MoreRead } from "./load-more";

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
