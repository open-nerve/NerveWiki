import { act, render, screen } from "@testing-library/react";
import { useRef } from "react";
import { expect, test, vi } from "vitest";

import { useFocusLeaving } from "./focus-leaving";

// An element leaving the document with the focus gives it elsewhere (v0.1
// design 13.2, item 17); one leaving without it takes nothing from where the
// reader put it (item 26).

function Leaving({ left }: { left: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useFocusLeaving(ref, left);
  return (
    <div ref={ref}>
      <button type="button">Inside</button>
    </div>
  );
}

function Shown({ on, left }: { on: boolean; left: () => void }) {
  return (
    <>
      <button type="button">Elsewhere</button>
      {on && <Leaving left={left} />}
    </>
  );
}

test("leaving with the focus in it, the element gives it away, by the latest given", () => {
  const first = vi.fn();
  const latest = vi.fn();
  const { rerender } = render(<Shown on left={first} />);
  act(() => screen.getByRole("button", { name: "Inside" }).focus());
  rerender(<Shown on left={latest} />);

  rerender(<Shown on={false} left={latest} />);
  expect([first.mock.calls.length, latest.mock.calls.length]).toEqual([0, 1]);
});

test("leaving with the focus elsewhere, the element gives nothing", () => {
  const left = vi.fn();
  const { rerender } = render(<Shown on left={left} />);
  const elsewhere = screen.getByRole("button", { name: "Elsewhere" });
  act(() => elsewhere.focus());

  rerender(<Shown on={false} left={left} />);
  expect(left).not.toHaveBeenCalled();
  expect(document.activeElement).toBe(elsewhere);
});
