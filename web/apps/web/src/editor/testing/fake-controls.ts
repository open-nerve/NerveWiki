import { vi } from "vitest";

import type { EditorControls } from "../registry";

/**
 * fakeControls are an extension's controls with the editor played by the
 * test: change is a change of the content, close the state going, which
 * ends the subscriptions; save and leave are spies that resolve unless the
 * test says otherwise.
 */
export function fakeControls() {
  const changed = new Set<() => void>();
  const closing: (() => void)[] = [];
  const controls = {
    save: vi.fn(() => Promise.resolve()),
    saving: () => false,
    setReadOnly: vi.fn(),
    session: () => ({ lost: false }),
    onSessionChange: () => () => undefined,
    onChange: (listener) => {
      changed.add(listener);
      return () => void changed.delete(listener);
    },
    onClose: (listener) => void closing.push(listener),
    leave: vi.fn((_reason: "idle") => Promise.resolve()),
    whenComposed: (act: () => void) => act(),
    tell: vi.fn((_text: string) => undefined),
  } satisfies EditorControls;
  return {
    controls,
    change: () => {
      for (const listener of changed) {
        listener();
      }
    },
    close: () => {
      changed.clear();
      for (const done of closing.splice(0)) {
        done();
      }
    },
  };
}
