import { EditorView } from "@codemirror/view";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";

import { I18nProvider } from "../i18n/i18n";
import { lineDiff, type LineDiffResult } from "../lib/line-diff";
import { TextDiff } from "./text-diff";

type Compare = (a: string, b: string, signal: AbortSignal) => Promise<LineDiffResult>;

/** The diff of before and after, in Chinese, the large comparisons run by compareLarge. */
const diff = (before: string, after: string, compareLarge?: Compare) => (
  <I18nProvider locale="zh-CN">
    <TextDiff
      before={before}
      after={after}
      label="比较"
      beforeName="版本 1"
      afterName="版本 2"
      {...(compareLarge === undefined ? {} : { compareLarge })}
    />
  </I18nProvider>
);

/** inline compares on the test's thread, as the worker would. */
const inline: Compare = (a, b) => Promise.resolve(lineDiff(a, b));

const lines = Array.from({ length: 12 }, (_, i) => `line ${i.toString()}\n`).join("");

/** text is lines, x, lines, y, lines: two changes apart. */
const text = (x: string, y: string) => `${lines}${x}\n${lines}${y}\n${lines}`;

/** paragraph is the paragraph i of a long page of some 120 characters each. */
const paragraph = (i: number) => `段落 ${i.toString()}：${"这是一段用来比较的文字，".repeat(10)}\n`;

describe("TextDiff", () => {
  test("says when the texts are the same, or differ in their line breaks or byte order mark alone", () => {
    const { rerender } = render(diff("a\nb\n", "a\nb\n"));
    expect(screen.getByText("没有改动。")).toBeDefined();
    rerender(diff("a\r\nb\r\n", `${String.fromCodePoint(0xfeff)}a\nb\n`));
    expect(screen.getByText("只差换行或 BOM。")).toBeDefined();
  });

  test("marks the lines added and removed, counts the changes and the lines, and gives the legend", () => {
    const { container } = render(diff(`${lines}old\n`, `${lines}new\nmore\n`));
    expect(screen.getByText("1 处改动")).toBeDefined();
    expect(screen.getByText("+2 −1 行")).toBeDefined();
    expect(screen.getByText("版本 1 → 版本 2")).toBeDefined();
    expect(container.querySelector(".cm-deletedLine")?.textContent).toBe("old");
    expect([...container.querySelectorAll(".cm-changedLine")].map((line) => line.textContent)).toEqual(["new", "more"]);
    expect(container.querySelector(".cm-collapsedLines")?.textContent).toMatch(/^\d+ 行未改动$/);
    expect(container.querySelector(".cm-content")?.getAttribute("aria-label")).toBe("比较");
  });

  test("moves to the next change and the one before", () => {
    const { container } = render(diff(text("a", "b"), text("c", "d")));
    expect(screen.getByText("2 处改动")).toBeDefined();
    const view = EditorView.findFromDOM(container.querySelector<HTMLElement>(".cm-editor") ?? document.body);
    const line = () => view?.state.doc.lineAt(view.state.selection.main.head).text;
    fireEvent.click(screen.getByRole("button", { name: "下一处" }));
    expect(line()).toBe("c");
    fireEvent.click(screen.getByRole("button", { name: "下一处" }));
    expect(line()).toBe("d");
    fireEvent.click(screen.getByRole("button", { name: "上一处" }));
    expect(line()).toBe("c");
  });

  test("compares large texts away from the page's thread, and drops a comparison it no longer shows", async () => {
    const signals: AbortSignal[] = [];
    const compareLarge: Compare = (a, b, signal) => {
      signals.push(signal);
      return inline(a, b, signal);
    };
    // M8 design review B-I1: some 30,000 characters with eight words changed show as eight changes.
    const before = Array.from({ length: 250 }, (_, i) => paragraph(i)).join("");
    let after = before;
    for (const i of [3, 40, 77, 110, 150, 190, 220, 249]) {
      after = after.replace(paragraph(i), paragraph(i).replace("用来比较", "供作对照"));
    }
    const { unmount } = render(diff(before, after, compareLarge));
    expect(screen.getByText("正在比较…")).toBeDefined();
    expect(await screen.findByText("8 处改动")).toBeDefined();
    unmount();
    expect(signals.map((s) => s.aborted)).toEqual([true]);
  });

  test("shows texts too far apart side by side, with how far when the comparison knows", async () => {
    const big = "x\n".repeat(20_000);
    const { rerender } = render(
      diff(big, `${big}y\n`, () => Promise.resolve({ kind: "tooLarge", added: 6000, removed: 10 }))
    );
    expect(await screen.findByText("+6000 −10 行，太大，不能逐行比较。")).toBeDefined();
    expect(screen.getByRole("heading", { name: "版本 1" })).toBeDefined();
    expect(screen.getByRole("heading", { name: "版本 2" })).toBeDefined();
    expect(document.querySelectorAll(".cm-editor")).toHaveLength(2);
    rerender(diff(big, `${big}z\n`, () => Promise.resolve({ kind: "tooLarge", added: null, removed: null })));
    expect(await screen.findByText("太大，不能逐行比较。")).toBeDefined();
  });

  test("shows a comparison only for the texts it compared", async () => {
    let finish: ((result: LineDiffResult) => void) | undefined;
    const pending: Compare = () =>
      new Promise((resolve) => {
        finish = resolve;
      });
    const big = "x\n".repeat(20_000);
    const { rerender } = render(diff(big, `${big}y\n`, pending));
    const first = finish;
    rerender(diff(big, `${big}z\n`, pending));
    await act(async () => {
      first?.({ kind: "tooLarge", added: 1, removed: 0 });
      await Promise.resolve();
    });
    expect(screen.queryByText(/太大/)).toBeNull();
    expect(screen.getByText("正在比较…")).toBeDefined();
  });
});
