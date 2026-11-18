import type { Locator, Page } from "@playwright/test";

import { uploadAsset, wav } from "../../fixtures/assets";
import { goto } from "../../fixtures/navigation";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// AS6, the reading view's links and media as a reader uses them (M12/P2
// design 4.5.0.3), in Chromium alone (P2-30): a control of an audio in a
// folded callout's title leaves the callout as it was, which only Chromium
// toggled (15.2).

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
