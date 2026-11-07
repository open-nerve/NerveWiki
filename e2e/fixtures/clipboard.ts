import { expect, type BrowserContext, type Locator, type Page } from "@playwright/test";

export type ClipboardBrowser = "chromium" | "firefox" | "webkit";

/** Firefox has no clipboard permission grant; its real paste supplies readback. */
export async function grantClipboardPermissions(context: BrowserContext, browserName: ClipboardBrowser): Promise<void> {
  if (browserName === "chromium") {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  } else if (browserName === "webkit") {
    await context.grantPermissions(["clipboard-read"]);
  } else if (browserName !== "firefox") {
    throw new Error(`unsupported clipboard browser: ${browserName}`);
  }
}

/**
 * Reads the actual clipboard after the story's real copy. A modal caller supplies
 * its active dialog as host so its focus trap also contains the Firefox probe.
 */
export async function readClipboardText(
  page: Page,
  browserName: ClipboardBrowser,
  host: Locator = page.locator("body")
): Promise<string> {
  if (browserName === "chromium" || browserName === "webkit") {
    return page.evaluate(() => navigator.clipboard.readText());
  }
  if (browserName !== "firefox") throw new Error(`unsupported clipboard browser: ${browserName}`);
  const probe = await host.evaluateHandle((element) => {
    const active = document.activeElement;
    const input = active instanceof HTMLInputElement || active instanceof HTMLTextAreaElement ? active : null;
    const inputSelection =
      input !== null && input.selectionStart !== null && input.selectionEnd !== null
        ? { start: input.selectionStart, end: input.selectionEnd, direction: input.selectionDirection }
        : null;
    const selection = window.getSelection();
    const ranges =
      selection === null
        ? []
        : Array.from({ length: selection.rangeCount }, (_, i) => selection.getRangeAt(i).cloneRange());
    const anchor = selection?.anchorNode ?? null;
    const focus = selection?.focusNode ?? null;
    const anchorOffset = selection?.anchorOffset ?? 0;
    const focusOffset = selection?.focusOffset ?? 0;
    const target = document.createElement("textarea");
    target.setAttribute("aria-label", "Clipboard readback probe");
    target.tabIndex = -1;
    let pasted: { trusted: boolean; hasData: boolean; text: string } | null = null;
    let inserted = false;
    const onPaste = (event: ClipboardEvent) => {
      pasted = {
        trusted: event.isTrusted,
        hasData: event.clipboardData !== null,
        text: event.clipboardData?.getData("text/plain") ?? "",
      };
    };
    const onInput = (event: Event) => {
      inserted = event.isTrusted;
    };
    target.addEventListener("paste", onPaste);
    target.addEventListener("input", onInput);
    return {
      start: () => {
        element.append(target);
        target.focus({ preventScroll: true });
      },
      read: () => ({ pasted, inserted, value: target.value }),
      restore: () => {
        target.removeEventListener("paste", onPaste);
        target.removeEventListener("input", onInput);
        target.remove();
        if (active instanceof HTMLElement) {
          if (!active.isConnected) throw new Error("the clipboard probe's original focus was removed");
          active.focus({ preventScroll: true });
        }
        if (selection !== null) {
          selection.removeAllRanges();
          for (const range of ranges) selection.addRange(range);
          if (ranges.length === 1 && anchor !== null && focus !== null) {
            selection.setBaseAndExtent(anchor, anchorOffset, focus, focusOffset);
          }
        }
        if (input !== null && inputSelection !== null) {
          input.setSelectionRange(inputSelection.start, inputSelection.end, inputSelection.direction ?? "none");
        }
      },
    };
  });
  try {
    await probe.evaluate((state) => state.start());
    await page.keyboard.press("ControlOrMeta+v");
    // Node-side polling still works when LW4 has installed the page's fake clock.
    await expect.poll(() => probe.evaluate((state) => state.read().inserted)).toBe(true);
    const { pasted, value } = await probe.evaluate((state) => state.read());
    expect(pasted?.trusted, "clipboard paste must be a trusted browser event").toBe(true);
    expect(pasted?.hasData, "clipboard paste must expose clipboardData").toBe(true);
    if (pasted === null || pasted.text.length === 0) throw new Error("the clipboard paste had no text");
    expect(value, "the browser must perform the default paste into the empty probe").toBe(pasted.text);
    return pasted.text;
  } finally {
    try {
      await probe.evaluate((state) => state.restore());
    } finally {
      await probe.dispose();
    }
  }
}
