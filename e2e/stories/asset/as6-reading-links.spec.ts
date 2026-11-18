import type { Locator, Page } from "@playwright/test";

import { uploadAsset, utf8, wav } from "../../fixtures/assets";
import { grantClipboardPermissions, readClipboardText } from "../../fixtures/clipboard";
import { goto } from "../../fixtures/navigation";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// AS6, the reading view's links and media as a reader uses them (M12/P2
// design 4.5.0.3), in Chromium alone (P2-30): a control of an audio in a
// folded callout's title leaves the callout as it was, which only Chromium
// toggled (15.2); a page's text copied leaves out the unseen hints of links
// that open in a tab of their own, which their names keep (17.2b).

const pdfBytes = utf8("%PDF-1.4\n% a document\n");
const zipBytes = Uint8Array.from([0x50, 0x4b, 0x03, 0x04, ...utf8("an archive")]);

/** frames waits for two frames of page: what a click changed is painted. */
async function frames(page: Page): Promise<void> {
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
}

/**
 * watchFrames counts, from now on, the frames of page painted with the
 * callout open (window.drawnOpen): a ResizeObserver's first delivery comes
 * in each frame after its animation frame callbacks and its layout, just
 * before it is painted, so what it reads is what is drawn (M12/P2/S5 4.5.0.5
 * item 2, the Step's probe).
 */
async function watchFrames(page: Page, callout: Locator): Promise<void> {
  await callout.evaluate((details) => {
    const counted = window as Window & { drawnOpen?: number };
    counted.drawnOpen = 0;
    const frame = () => {
      const observer = new ResizeObserver(() => {
        observer.disconnect();
        if ((details as HTMLDetailsElement).open) {
          counted.drawnOpen = (counted.drawnOpen ?? 0) + 1;
        }
      });
      observer.observe(document.body);
      requestAnimationFrame(frame);
    };
    requestAnimationFrame(frame);
  });
}

test("AS6 (page): a control of an audio in a folded callout's title leaves the callout as it was", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { tokens, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const sound = await uploadAsset(api, pat, notebook.id, { name: "sound.wav", bytes: wav(3) });
  const guide = await createPage(api, pat, notebook.id, "Guide", null, "> [!note]- ![[sound.wav]] Talk\n> body\n");
  await goto(page, wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(pageHeading(page, "Guide")).toBeVisible();
  const view = page.getByRole("article", { name: "Guide" });
  const callout = view.locator("details.nw-callout");
  const audio = callout.locator("summary audio.nw-asset");
  const body = view.getByText("body", { exact: true });
  // Its address signed; its data not read (preload="none"), as the reader first has it.
  await expect(audio).toBeVisible();
  await expect(audio).toHaveAttribute("src", new RegExp(`^/api/v0/assets/${sound.id}/content\\?.+`));
  await watchFrames(page, callout);

  // The control's blank, left of its button, and its time: Chromium toggled the callout for either.
  const box = await audio.boundingBox();
  const clickAt = async (x: number) => {
    await audio.click({ position: { x, y: (box?.height ?? 0) / 2 } });
    await frames(page);
    expect(await callout.evaluate((details) => (details as HTMLDetailsElement).open), `after a click at ${x}`).toBe(
      false
    );
    await expect(body).toBeHidden();
  };
  await clickAt(6);
  await clickAt(60);
  // Not a frame was drawn open: the callout is put back before the frame is painted.
  expect(await page.evaluate(() => (window as Window & { drawnOpen?: number }).drawnOpen)).toBe(0);

  // The title's text toggles it as ever.
  await page.mouse.click((box?.x ?? 0) + (box?.width ?? 0) + 16, (box?.y ?? 0) + (box?.height ?? 0) / 2);
  await expect(body).toBeVisible();
  expect(await callout.evaluate((details) => (details as HTMLDetailsElement).open)).toBe(true);
});

/** copied is the text of element as the reader copies it: selected whole, then ControlOrMeta+C. */
async function copied(page: Page, element: Locator): Promise<string> {
  await element.evaluate((node) => {
    const range = document.createRange();
    range.selectNodeContents(node);
    window.getSelection()?.removeAllRanges();
    window.getSelection()?.addRange(range);
  });
  await page.keyboard.press("ControlOrMeta+c");
  return readClipboardText(page, "chromium");
}

test("AS6 (page): copying a page's text leaves out the hints of links that open in a tab of their own, which their names keep: in the reading view, the properties and the attachments' rows", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { tokens, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  await grantClipboardPermissions(page.context(), "chromium");
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(
    api,
    pat,
    notebook.id,
    "Guide",
    null,
    '---\ncover: "[[doc.pdf]]"\n---\n[[doc.pdf]] [[data.zip|the data]] end\n'
  );
  await uploadAsset(api, pat, notebook.id, { name: "doc.pdf", bytes: pdfBytes }, guide.id);
  await uploadAsset(api, pat, notebook.id, { name: "data.zip", bytes: zipBytes }, guide.id);
  await goto(page, wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(pageHeading(page, "Guide")).toBeVisible();
  const view = page.getByRole("article", { name: "Guide" });
  const properties = page.getByRole("complementary", { name: "About this page" });
  const section = page.getByRole("region", { name: "Attachments" });

  // Each link's name says it opens in a tab of its own: the paragraph's (the view's table of properties has one too),
  // the properties' and the row's.
  const paragraph = view.locator("p").filter({ hasText: "end" });
  const named = { name: "doc.pdf (opens in a new tab)", exact: true };
  await Promise.all(
    [paragraph, properties, section].map((where) => expect(where.getByRole("link", named)).toBeVisible())
  );

  // Copied, the text has none of it: the paragraph as it reads, its sizes, one space apart.
  await expect(paragraph).toContainText(`(${zipBytes.length.toString()} B)`);
  expect(await copied(page, paragraph)).toBe(
    `doc.pdf (${pdfBytes.length.toString()} B) the data (${zipBytes.length.toString()} B) end`
  );
  const value = await copied(page, properties.getByRole("link", named).locator(".."));
  expect(value).toContain("doc.pdf");
  expect(value).not.toContain("(opens in a new tab)");
  const rows = await copied(page, section.getByRole("list"));
  expect(rows).toContain("doc.pdf");
  expect(rows).not.toContain("(opens in a new tab)");
});
