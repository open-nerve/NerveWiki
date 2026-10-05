import { expect, test, vi } from "vitest";

import { enhance, type Enhancement, type ReadingContext } from "./enhancement";
import { translator } from "../i18n/i18n";

const context: ReadingContext = {
  workspace: "lab",
  notebook: "n",
  page: "p",
  revision: 1,
  role: "editor",
  t: translator("en"),
  theme: "light",
  reload: () => undefined,
  navigate: () => undefined,
  report: () => undefined,
  unresolved: () => undefined,
};

const throws: Enhancement = () => {
  throw new Error("run");
};

const undoThrows: Enhancement = () => () => {
  throw new Error("undo");
};

test("an enhancement that throws, running or undone, is logged and leaves the others be", () => {
  const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const log: string[] = [];
  const records: Enhancement = () => {
    log.push("run");
    return () => log.push("undo");
  };

  const undo = enhance([throws, records, undoThrows], document.createElement("div"), context);
  expect(log).toEqual(["run"]);
  undo();

  expect(log).toEqual(["run", "undo"]);
  expect(error).toHaveBeenCalledTimes(2);
});
