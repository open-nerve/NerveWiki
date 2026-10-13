import { EditorView } from "@codemirror/view";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, onTestFinished, test, vi } from "vitest";

import { I18nProvider } from "../i18n/i18n";
import { lineDiff, type LineDiffResult } from "../lib/line-diff";
import { compareInWorker, type DiffWorker } from "../lib/line-diff-client";
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

/** controlled is a comparison that answers, or fails, when the test says: each one asked, with its signal. */
function controlled() {
  const asked: { signal: AbortSignal; finish: (result: LineDiffResult) => void; fail: () => void }[] = [];
  const compare: Compare = (_a, _b, signal) =>
    new Promise((resolve, reject) => {
      asked.push({ signal, finish: resolve, fail: () => reject(new Error("failed")) });
    });
  return { compare, asked };
}

/** answered has the comparison asked finish, or fail, and what it answers show. */
async function answered(asked: ReturnType<typeof controlled>["asked"], nth: number, result?: LineDiffResult) {
  await act(async () => {
    if (result === undefined) {
      asked[nth]?.fail();
    } else {
      asked[nth]?.finish(result);
    }
    await Promise.resolve();
  });
}

const lines = Array.from({ length: 12 }, (_, i) => `line ${i.toString()}\n`).join("");

/** text is lines, x, lines, y, lines: two changes apart. */
const text = (x: string, y: string) => `${lines}${x}\n${lines}${y}\n${lines}`;

/** paragraph is the paragraph i of a long page of some 120 characters each. */
const paragraph = (i: number) => `段落 ${i.toString()}：${"这是一段用来比较的文字，".repeat(10)}\n`;

/** said is what a failure says, and whether it offers Reload. */
const said = () => ({
  alert: screen.queryByRole("alert")?.textContent,
  reload: screen.queryByRole("button", { name: "重新加载" }) !== null,
});

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

  test("shows a comparison only for the texts it compared, dropping the one before", async () => {
    const { compare, asked } = controlled();
    const big = "x\n".repeat(20_000);
    const { rerender } = render(diff(big, `${big}y\n`, compare));
    rerender(diff(big, `${big}z\n`, compare));
    expect(asked.map((each) => each.signal.aborted)).toEqual([true, false]);
    await act(async () => {
      asked[0]?.finish({ kind: "tooLarge", added: 1, removed: 0 });
      await Promise.resolve();
    });
    expect(screen.queryByText(/太大/)).toBeNull();
    expect(screen.getByText("正在比较…")).toBeDefined();
  });

  // M8/P1 review A, M4: a page edited elsewhere is read again every few seconds.
  test("keeps the comparison it shows, marked busy, while that of newer texts runs", async () => {
    const { compare, asked } = controlled();
    const big = "x\n".repeat(20_000);
    const { container, rerender } = render(diff(big, `${big}y\n`, compare));
    await act(async () => {
      asked[0]?.finish(lineDiff(big, `${big}y\n`));
      await Promise.resolve();
    });
    const next = screen.getByRole("button", { name: "下一处" });
    next.focus();
    rerender(diff(big, `${big}y\nz\n`, compare));
    expect(screen.getByText("+1 −0 行")).toBeDefined();
    expect(container.querySelector("[aria-busy=true]")).not.toBeNull();
    // Seen as well as heard: it says it compares, the view dimmed.
    expect(screen.getByText("正在比较…")).toBeDefined();
    expect(container.querySelector(".opacity-60 .cm-editor")).not.toBeNull();
    expect(document.activeElement).toBe(next);
    await act(async () => {
      asked[1]?.finish(lineDiff(big, `${big}y\nz\n`));
      await Promise.resolve();
    });
    expect(screen.getByText("+2 −0 行")).toBeDefined();
    expect(container.querySelector("[aria-busy]")).toBeNull();
    expect(screen.queryByText("正在比较…")).toBeNull();
    expect(container.querySelector(".opacity-60")).toBeNull();
    expect(document.activeElement).toBe(next);
  });

  test("keeps texts too far apart side by side, saying it compares, while newer texts compare", async () => {
    const { compare, asked } = controlled();
    const big = "x\n".repeat(20_000);
    const { container, rerender } = render(diff(big, `${big}y\n`, compare));
    await act(async () => {
      asked[0]?.finish({ kind: "tooLarge", added: null, removed: null });
      await Promise.resolve();
    });
    rerender(diff(big, `${big}z\n`, compare));
    expect(screen.getByText("太大，不能逐行比较。")).toBeDefined();
    expect(screen.getByText("正在比较…")).toBeDefined();
    expect(container.querySelector("[aria-busy=true]")).not.toBeNull();
  });

  test("lets an earlier comparison go once the texts are small enough to compare at once", async () => {
    const { compare, asked } = controlled();
    const big = "x\n".repeat(20_000);
    const { rerender } = render(diff(big, `${big}y\n`, compare));
    await act(async () => {
      asked[0]?.finish({ kind: "tooLarge", added: 1, removed: 0 });
      await Promise.resolve();
    });
    rerender(diff("a\n", "b\n", compare));
    expect(screen.getByText("1 处改动")).toBeDefined();
    rerender(diff(big, `${big}z\n`, compare));
    expect(screen.queryByText(/太大/)).toBeNull();
    expect(screen.getByText("正在比较…")).toBeDefined();
  });

  // M8/P1 review A, M3: a worker whose script an upgrade removed, or none at all.
  test("says when the comparison failed, and compares again on Try again", async () => {
    let fail = true;
    let answer: (() => void) | undefined;
    const compare: Compare = (a, b, signal) =>
      compareInWorker(a, b, signal, () => {
        if (fail) {
          throw new Error("no workers here");
        }
        const answering: DiffWorker = {
          addEventListener: (type: string, listener: (event: MessageEvent<LineDiffResult>) => void) => {
            if (type === "message") {
              answer = () => listener(new MessageEvent("message", { data: lineDiff(a, b) }));
            }
          },
          postMessage: () => undefined,
          terminate: () => undefined,
        };
        return answering;
      });
    const big = "x\n".repeat(20_000);
    render(diff(big, `${big}y\n`, compare));
    expect((await screen.findByRole("alert")).textContent).toBe("比较失败。");
    expect(screen.queryByRole("button", { name: "重新加载" })).toBeNull();
    fail = false;
    const retry = screen.getByRole("button", { name: "重试" });
    retry.focus();
    fireEvent.click(retry);
    // M8/P1 fix check 3, R3-2: the failure stays as the comparison runs again, saying so; Try again keeps the focus.
    expect(screen.getByRole("alert").textContent).toBe("正在比较…");
    expect(retry.getAttribute("aria-disabled")).toBe("true");
    expect(document.activeElement).toBe(retry);
    act(() => answer?.());
    expect(await screen.findByText("+1 −0 行")).toBeDefined();
    // Gone with the focus, the failure gives it to the diff: not to the page's start.
    expect(screen.queryByRole("alert")).toBeNull();
    // M8/P1 fix check 4, R4-2: a group named as the diff is, not by all the diff's text.
    const group = screen.getByRole("group", { name: "比较" });
    expect(document.activeElement).toBe(group);
    expect(group.contains(screen.getByRole("button", { name: "下一处" }))).toBe(true);
  });

  // M8/P1 fix check 3, R3-2 (v0.1 design 13.2, items 12 and 17).
  test("Try again, comparing, does nothing and keeps the focus; failed again, Reload follows it, which Tab reaches", async () => {
    const user = userEvent.setup();
    const { compare, asked } = controlled();
    const big = "x\n".repeat(20_000);
    render(diff(big, `${big}y\n`, compare));
    await answered(asked, 0);
    const retry = screen.getByRole("button", { name: "重试" });
    await user.click(retry);
    expect(screen.getByRole("alert").textContent).toBe("正在比较…");
    await user.click(retry);
    expect(asked).toHaveLength(2);
    expect(document.activeElement).toBe(retry);

    await answered(asked, 1);
    expect(screen.getByRole("alert").textContent).toBe("比较又失败了。重新加载页面通常能解决。");
    expect(retry.hasAttribute("aria-disabled")).toBe(false);
    expect(document.activeElement).toBe(retry);
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "重新加载" }));
  });

  // M8/P1 fix check 3, R3-3: Reload is offered for a failure of the same texts again.
  test("says it failed again only when the same texts' comparison before failed too: a success, or newer texts, start anew", async () => {
    const { compare, asked } = controlled();
    const big = "x\n".repeat(20_000);
    const { rerender } = render(diff(big, `${big}y\n`, compare));
    await answered(asked, 0);
    expect(said()).toEqual({ alert: "比较失败。", reload: false });
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await answered(asked, 1);
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await answered(asked, 2);
    expect(said()).toEqual({ alert: "比较又失败了。重新加载页面通常能解决。", reload: true });

    // Newer texts, whose first comparison fails.
    rerender(diff(big, `${big}z\n`, compare));
    await answered(asked, 3);
    expect(said()).toEqual({ alert: "比较失败。", reload: false });
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await answered(asked, 4);
    expect(said()).toEqual({ alert: "比较又失败了。重新加载页面通常能解决。", reload: true });

    // M8/P1 fix check 4, N1: the text before alone newer, as the page now is in the restore dialog.
    rerender(diff(`${big}q\n`, `${big}z\n`, compare));
    await answered(asked, 5);
    expect(said()).toEqual({ alert: "比较失败。", reload: false });

    // A success, then a failure of newer texts.
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await answered(asked, 6, lineDiff(`${big}q\n`, `${big}z\n`));
    expect(screen.getByText("+1 −1 行")).toBeDefined();
    expect(document.activeElement).toBe(document.body);
    rerender(diff(`${big}q\n`, `${big}w\n`, compare));
    await answered(asked, 7);
    expect(said()).toEqual({ alert: "比较失败。", reload: false });
  });

  test("Reload, gone with the focus as newer texts fail but once, gives it to Try again", async () => {
    const { compare, asked } = controlled();
    const big = "x\n".repeat(20_000);
    const { rerender } = render(diff(big, `${big}y\n`, compare));
    await answered(asked, 0);
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await answered(asked, 1);
    const reload = screen.getByRole("button", { name: "重新加载" });
    reload.focus();
    rerender(diff(big, `${big}z\n`, compare));
    expect(document.activeElement).toBe(reload);
    await answered(asked, 2);
    expect(screen.queryByRole("button", { name: "重新加载" })).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "重试" }));
  });

  // M8/P1 fix check 2, R2-2: a worker of an older build than the server's fails each time; a reload replaces it.
  test("failed again on Try again, says a reload of the page usually helps, with Reload", async () => {
    const big = "x\n".repeat(20_000);
    render(diff(big, `${big}y\n`, () => Promise.reject(new Error("failed"))));
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    expect((await screen.findByRole("alert")).textContent).toBe("比较又失败了。重新加载页面通常能解决。");
    const reload = vi.fn();
    vi.stubGlobal("location", { ...window.location, reload });
    onTestFinished(() => {
      vi.unstubAllGlobals();
    });
    fireEvent.click(screen.getByRole("button", { name: "重新加载" }));
    expect(reload).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: "重试" })).toBeDefined();
  });

  test("says nothing of a failure while the comparison of newer texts runs: that it compares, Try again doing nothing", async () => {
    const { compare, asked } = controlled();
    const big = "x\n".repeat(20_000);
    const { container, rerender } = render(diff(big, `${big}y\n`, compare));
    await answered(asked, 0);
    expect(screen.getByRole("alert").textContent).toBe("比较失败。");
    rerender(diff(big, `${big}z\n`, compare));
    expect(screen.getByRole("alert").textContent).toBe("正在比较…");
    expect(container.querySelector("[aria-busy=true]")).not.toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    expect(asked).toHaveLength(2);
    await answered(asked, 1, lineDiff(big, `${big}z\n`));
    expect(screen.getByText("+1 −0 行")).toBeDefined();
    expect(container.querySelector("[aria-busy]")).toBeNull();
  });

  // M8/P1 review A, M5: the view joins the changes that only an empty line parts, and shows a change's lines removed at once.
  test("shows side by side changes the view would show as more than 5,000 lines removed in one block", async () => {
    // 2,501 paragraphs removed, the empty lines between them kept: one block of 5,001 lines.
    const before = Array.from({ length: 2501 }, (_, i) => `paragraph ${i.toString()}\n`).join("\n");
    const after = "\n".repeat(2500);
    render(diff(before, after, inline));
    expect(await screen.findByText("+0 −2501 行，太大，不能逐行比较。")).toBeDefined();
    expect(document.querySelectorAll(".cm-editor")).toHaveLength(2);
  });

  test("shows a block of 5,000 lines removed in the unified view", async () => {
    const before = Array.from({ length: 5000 }, (_, i) => `line ${i.toString()}\n`).join("");
    render(diff(before, "", inline));
    expect(await screen.findByText("+0 −5000 行")).toBeDefined();
    expect(document.querySelectorAll(".cm-editor")).toHaveLength(1);
  });
});
