import { act, renderHook } from "@testing-library/react";
import { expect, test, vi } from "vitest";

import { useNarrow } from "./narrow";

/** phone stubs matchMedia: each query answers narrow, which change changes, telling the query's listeners. */
function phone(narrow: boolean) {
  let matches = narrow;
  const listeners = new Set<() => void>();
  const asked: string[] = [];
  vi.stubGlobal("matchMedia", (query: string) => {
    asked.push(query);
    return {
      get matches() {
        return matches;
      },
      addEventListener: (_type: string, listener: () => void) => listeners.add(listener),
      removeEventListener: (_type: string, listener: () => void) => listeners.delete(listener),
    };
  });
  return {
    asked,
    change: (to: boolean) => {
      matches = to;
      for (const listener of listeners) listener();
    },
  };
}

// The width below md, Tailwind's max-md: (M13/P2/S1, decision 801; the drawer's too, M13/P2/S3).

test("without matchMedia, as jsdom has none, it is not narrow", () => {
  expect(typeof window.matchMedia).toBe("undefined");
  expect(renderHook(() => useNarrow()).result.current).toBe(false);
});

test("it answers md's query, (width < 48rem), and follows its changes", () => {
  const media = phone(true);
  const { result } = renderHook(() => useNarrow());
  expect(result.current).toBe(true);
  expect(new Set(media.asked)).toEqual(new Set(["(width < 48rem)"]));

  act(() => media.change(false));
  expect(result.current).toBe(false);
  act(() => media.change(true));
  expect(result.current).toBe(true);
});
