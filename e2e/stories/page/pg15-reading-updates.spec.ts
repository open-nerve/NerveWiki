import type { ApiClient, AuthTokens } from "@nervewiki/api-client";
import type { Page, TestInfo } from "@playwright/test";
import { uploadAsset, vp8Webm, wav } from "../../fixtures/assets";
import { answerTo, countAnswers } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { goto } from "../../fixtures/navigation";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// PG15, the reading view updated in place (M12/P2/S4, design 4.4.6): a
// view read again, another's write in it, is written where it changed, the
// rest as the reader has it, in each of the three engines: the focus, a
// callout opened, the selection, an audio playing, a video playing in fullscreen;
// a heading of the same name put before the focused one leaves the focus on
// it. The write comes through the API, with the account's own token: to the
// view it is a write elsewhere, which the pages event reads again. Back at
// an address with an anchor, its heading shows, the focus in view (8.1).

/** The page's head, to which each cell adds what it looks at; filler makes it long. */
const head = "[[Other|to other]]\n\n> [!note]- Folded\n> inside\n\nWords to select in this paragraph.\n\n";
const filler = "filler\n\n".repeat(40);

/**
 * opened is Plans' page Read of content, attachments uploaded first, open in a page signed in as the team's admin, its
 * stream connected and its refresh in: from then on, a write comes as an event. write writes the page anew through
 * the API and waits for the next answer of the view and for the write's text (review B's N1: the answer may be of
 * a read already under way; the text is the write's).
 */
async function opened(
  { api, signedInPage }: { api: ApiClient; signedInPage: (tokens: AuthTokens) => Promise<Page> },
  testInfo: TestInfo,
  content: string,
  attachments: (page: Page, pat: string, notebook: string) => Promise<void> = async () => undefined
) {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const page = await signedInPage(tokens);
  await goto(page, `/${workspace.slug}/notebooks/${notebook.id}`);
  await attachments(page, pat, notebook.id);
  await createPage(api, pat, notebook.id, "Other");
  const read = await createPage(api, pat, notebook.id, "Read", null, content);
  const viewPath = `/api/v0/pages/${read.id}/view`;
  const letStreamIn = await holdStream(page);
  const viewReads = countAnswers(page, "GET", viewPath);
  await goto(page, wikiPagePath(workspace.slug, notebook.id, read.id));
  const article = page.getByRole("article", { name: "Read" });
  await expect(article.getByRole("link", { name: "to other" })).toBeVisible();
  letStreamIn();
  await expect.poll(viewReads).toBeGreaterThanOrEqual(2);
  let revision = read.revision;
  const write = async (next: string, shows: string) => {
    const answered = answerTo(page, "GET", viewPath);
    revision = (await writeContent(api, pat, read.id, { content: next, base_revision: revision })).revision;
    expect((await answered).status()).toBe(200);
    await expect(article.getByText(shows, { exact: true })).toBeVisible();
  };
  return { page, article, write };
}

/** marked keeps on page's window, as name, the article's first child: the same node, it is still. */
function marked(page: Page, name: string): Promise<void> {
  return page.evaluate((key) => {
    Object.assign(window, { [key]: document.querySelector("article")?.firstChild });
  }, name);
}

/** isMarked tells whether the article's first child is the one marked as name. */
function isMarked(page: Page, name: string): Promise<boolean> {
  return page.evaluate((key) => document.querySelector("article")?.firstChild === Reflect.get(window, key), name);
}

test("PG15 (page): the reading view updated in place: another's write elsewhere in the page keeps the focus on a link, a callout opened, the selection and an audio playing", async ({
  api,
  signedInPage,
}, testInfo) => {
  const content = `${head}![[talk.wav]]\n\n${filler}The end.\n`;
  const { page, article, write } = await opened({ api, signedInPage }, testInfo, content, async (_page, pat, id) => {
    await uploadAsset(api, pat, id, { name: "talk.wav", bytes: wav(30) });
  });
  await article.getByText("Folded", { exact: true }).click();
  await expect(article.getByText("inside", { exact: true })).toBeVisible();
  const audio = article.locator("audio.nw-asset");
  await audio.evaluate(async (element) => {
    const media = element as HTMLAudioElement;
    media.muted = true;
    await media.play();
    Object.assign(window, { playing: media });
  });
  const link = article.getByRole("link", { name: "to other" });
  await link.focus();
  await article.getByText("Words to select in this paragraph.").evaluate((paragraph) => {
    const words = paragraph.firstChild as Text;
    document.getSelection()?.setBaseAndExtent(words, 0, words, 5);
  });
  await marked(page, "first");
  await expect(link).toBeFocused();
  await expect.poll(() => page.evaluate(() => document.getSelection()?.toString())).toBe("Words");

  await write(content.replace("The end.", "The end, written again."), "The end, written again.");

  await expect(link).toBeFocused();
  await expect(article.locator("details.nw-callout")).toHaveAttribute("open", "");
  expect(await page.evaluate(() => document.getSelection()?.toString())).toBe("Words");
  expect(await isMarked(page, "first")).toBe(true);
  expect(
    await audio.evaluate((element) => {
      const media = element as HTMLAudioElement;
      return { same: media === Reflect.get(window, "playing"), paused: media.paused, at: media.currentTime > 0 };
    })
  ).toEqual({ same: true, paused: false, at: true });
});

test("PG15 (page): a heading of the same name put before the focused one leaves the focus on it, its section after it", async ({
  api,
  signedInPage,
}, testInfo) => {
  const notes = "## Notes\n\nThe notes we had.\n";
  const content = `${head}[[#Notes|to notes]]\n\n${filler}${notes}`;
  const { page, article, write } = await opened({ api, signedInPage }, testInfo, content);
  await article.getByRole("link", { name: "to notes" }).click();
  const focused = page.locator(":focus");
  await expect(focused).toHaveText("Notes");
  await page.evaluate(() => Object.assign(window, { notes: document.activeElement }));

  await write(content.replace(notes, `## Notes\n\nNew notes.\n\n${notes}`), "New notes.");

  expect(await page.evaluate(() => document.activeElement === Reflect.get(window, "notes"))).toBe(true);
  await expect(focused).toHaveAttribute("id", "nw-notes-1");
  expect(await focused.evaluate((heading) => heading.nextElementSibling?.textContent)).toBe("The notes we had.");
  await expect(focused).toBeInViewport();
});

test("PG15 (page): a video playing in fullscreen goes on in it as another's write elsewhere is read", async ({
  api,
  signedInPage,
}, testInfo) => {
  // A VP8 WebM recorded once, which each engine plays (review B's M2): the video begun is what the enhancement
  // would put anew (adopt), the node kept what keeps the fullscreen.
  const content = `${head}![[clip.webm]]\n\n${filler}The end.\n`;
  const { article, write } = await opened({ api, signedInPage }, testInfo, content, async (_page, pat, id) => {
    await uploadAsset(api, pat, id, { name: "clip.webm", bytes: vp8Webm });
  });
  const video = article.locator("video.nw-asset");
  await video.evaluate(async (element) => {
    const media = element as HTMLVideoElement;
    media.muted = true;
    media.loop = true;
    await media.play();
    await media.requestFullscreen();
    Object.assign(window, { playing: media });
  });
  await expect.poll(() => video.evaluate((element) => document.fullscreenElement === element)).toBe(true);
  await expect.poll(() => video.evaluate((element) => (element as HTMLVideoElement).currentTime > 0)).toBe(true);

  await write(content.replace("The end.", "The end, written again."), "The end, written again.");

  expect(
    await video.evaluate((element) => ({
      same: element === Reflect.get(window, "playing"),
      fullscreen: document.fullscreenElement === element,
      paused: (element as HTMLVideoElement).paused,
    }))
  ).toEqual({ same: true, fullscreen: true, paused: false });
});

// M12/P2/S4, 8.1 (step 0's probe, its far shape): back at an address with an anchor, the browser puts back the scroll
// the page had as it was left, after the view went to the anchor's heading: a reader who scrolled away before leaving
// came back to a heading with the focus out of view.
test("PG15 (page): back to an address with an anchor, its heading has the focus in view, though the reader scrolled away before leaving", async ({
  api,
  signedInPage,
}, testInfo) => {
  const content = `[[Other|to other]] [[#Part A|to part a]]\n\n${"filler\n\n".repeat(80)}# Part A\n\n${"after\n\n".repeat(40)}`;
  const { page, article } = await opened({ api, signedInPage }, testInfo, content);
  await article.getByRole("link", { name: "to part a" }).click();
  const partA = article.getByRole("heading", { name: "Part A", exact: true });
  await expect(partA).toBeFocused();
  await expect(partA).toBeInViewport();
  await page.evaluate(() => scrollTo(0, 0));
  await expect(article.getByRole("link", { name: "to other" })).toBeInViewport();
  await article.getByRole("link", { name: "to other" }).click();
  await expect(pageHeading(page, "Other")).toBeFocused();

  await page.goBack();

  await expect(partA).toBeFocused();
  await expect(partA).toBeInViewport();
  // And stays: the browser's own scroll came before the frame the view looked again at.
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  await expect(partA).toBeInViewport();
});
