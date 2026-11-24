import { render } from "@testing-library/react";
import { expect, test } from "vitest";

import { ColumnTop, ColumnTopContext } from "./column-top";

// What a shell puts at the top of its page's column (M13/P4/S1, decision 857).

test("without a provider the column's top is nothing; with one, what it gives", () => {
  const { container, rerender } = render(<ColumnTop />);
  expect(container.innerHTML).toBe("");

  rerender(
    <ColumnTopContext value={<output>Upgrading</output>}>
      <ColumnTop />
    </ColumnTopContext>
  );
  expect(container.innerHTML).toBe("<output>Upgrading</output>");
});
