import { formatBytes } from "../i18n/format";
import type { Enhancement, ReadingContext } from "./enhancement";

/** How many audios and videos a view keeps playing as its HTML is replaced: as many as it writes (obsidian's MaxMedia). */
const maxKept = 20;

/** The attachments' audios and videos a view writes. */
const media = "audio.nw-asset, video.nw-asset";

/**
 * assets is the attachments of the reading view (M7/P4 design 4.4–4.6),
 * whose HTML has them at their contents' addresses, signed for an hour or
 * more:
 *
 * - A link to one says its size after it, in the reader's language. One
 *   the browser shows opens in a tab of its own, as it says unseen; one to
 *   any other downloads it (the server writes download).
 * - An image, an audio or a video that fails to load, once the view's
 *   addresses have expired, reads the view again: once for each expiry of
 *   the page's, as one that fails for another reason would fail as much
 *   in the view read again.
 * - An audio or a video started (playing, or paused partway) is kept as
 *   the HTML is replaced, which the signatures do each hour and others'
 *   writes at any time: the next HTML of the page has it in place of its
 *   own of the same attachment, the same of them in order, its attributes
 *   but its address, the focus if it had it; one it does not have goes.
 *   One that failed is not kept: it would not load again, and its place
 *   goes to the new one. One kept has the address it had: as it fails
 *   (its address expired, as a rule), the view has the attachment's
 *   signed anew (assetAddress), and the media goes on where it was; once
 *   for each HTML, none for an attachment gone.
 */
export function assets(): Enhancement {
  // What the view had started as its HTML was replaced: its page, its audios and videos by key, and the focused one.
  let kept: Kept | undefined;
  // The expiry each page's view was read again for, as its attachments failed to load.
  const reloadedFor = new Map<string, string>();
  return (container, context) => {
    const adopted = adopt(container, kept?.page === context.page ? kept : undefined);
    kept = undefined;
    const links = newTab(container, context.t("asset.newTab"));
    const added = [...links.flatMap(({ hint }) => hint), ...sizes(container, context)];
    const onError = (event: Event) => {
      const element = event.target;
      if (!(element instanceof HTMLElement) || !element.matches(`img.nw-asset, ${media}`)) {
        return;
      }
      if (element instanceof HTMLMediaElement && adopted.delete(element)) {
        void signAnew(element, context);
        return;
      }
      const expires = context.assetsExpire;
      if (expires !== null && Date.now() >= Date.parse(expires) && reloadedFor.get(context.page) !== expires) {
        reloadedFor.set(context.page, expires);
        context.reload();
      }
    };
    // An element's error goes no further than it: the container hears it on its way down.
    container.addEventListener("error", onError, true);
    return () => {
      container.removeEventListener("error", onError, true);
      for (const { link } of links) {
        link.removeAttribute("target");
        link.removeAttribute("rel");
      }
      for (const node of added) {
        node.remove();
      }
      const playing = started(container);
      const active = document.activeElement;
      kept = {
        page: context.page,
        media: playing,
        focused: [...playing.values()].find((element) => element === active),
      };
    };
  };
}

/** Kept is what a view had started as its HTML was replaced: its page, its audios and videos by key, the focused one. */
type Kept = { page: string; media: Map<string, HTMLMediaElement>; focused: HTMLMediaElement | undefined };

/**
 * newTab has the links of container to attachments the browser shows open in a tab of their own, saying so unseen:
 * each link, and its hint, the space before it outside, which a name joins the words by.
 */
function newTab(container: HTMLElement, text: string): { link: HTMLAnchorElement; hint: ChildNode[] }[] {
  const added: { link: HTMLAnchorElement; hint: ChildNode[] }[] = [];
  for (const link of container.querySelectorAll<HTMLAnchorElement>("a.nw-asset[href]:not([download])")) {
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    const hint = document.createElement("span");
    hint.className = "sr-only";
    hint.textContent = text;
    const space = document.createTextNode(" ");
    link.append(space, hint);
    added.push({ link, hint: [space, hint] });
  }
  return added;
}

/**
 * sizes says, after each link of container to an attachment, its size in the reader's language: data-nw-size, which
 * the server writes in bytes with the link's address.
 */
function sizes(container: HTMLElement, { t, locale }: ReadingContext): HTMLElement[] {
  const added: HTMLElement[] = [];
  for (const link of container.querySelectorAll<HTMLAnchorElement>("a.nw-asset[data-nw-size]")) {
    const size = document.createElement("span");
    size.className = "nw-size";
    size.textContent = t("asset.sizeAfter", { size: formatBytes(Number(link.dataset.nwSize), locale) });
    link.after(size);
    added.push(size);
  }
  return added;
}

/**
 * adopt puts each of kept, by key, in place of the element of container
 * of the same key (an attachment's type is its own: the same element),
 * its attributes but its address taken, the focus given back if it had
 * it; those put are the adopted. One that failed is not put: the new one
 * starts where it was.
 */
function adopt(container: HTMLElement, kept: Kept | undefined): Set<HTMLMediaElement> {
  const adopted = new Set<HTMLMediaElement>();
  if (kept === undefined) {
    return adopted;
  }
  for (const [key, element] of keyed(container)) {
    const old = kept.media.get(key);
    if (old === undefined) {
      continue;
    }
    if (old.error !== null) {
      element.currentTime = old.currentTime;
      continue;
    }
    for (const name of old.getAttributeNames()) {
      if (name !== "src" && !element.hasAttribute(name)) {
        old.removeAttribute(name);
      }
    }
    for (const { name, value } of element.attributes) {
      if (name !== "src" && old.getAttribute(name) !== value) {
        old.setAttribute(name, value);
      }
    }
    element.replaceWith(old);
    adopted.add(old);
    if (old === kept.focused) {
      old.focus({ preventScroll: true });
    }
  }
  return adopted;
}

/** started are the audios and videos of container started and not ended, by key: the first maxKept of them. */
function started(container: HTMLElement): Map<string, HTMLMediaElement> {
  const out = new Map<string, HTMLMediaElement>();
  for (const [key, element] of keyed(container)) {
    if (out.size < maxKept && (!element.paused || element.currentTime > 0) && !element.ended) {
      out.set(key, element);
    }
  }
  return out;
}

/** keyed is each audio and video of container by its key: its attachment's id and its place among those of it. */
function keyed(container: HTMLElement): Map<string, HTMLMediaElement> {
  const out = new Map<string, HTMLMediaElement>();
  const counts = new Map<string, number>();
  for (const element of container.querySelectorAll<HTMLMediaElement>(media)) {
    const id = assetOf(element.getAttribute("src"));
    if (id !== undefined) {
      const nth = counts.get(id) ?? 0;
      counts.set(id, nth + 1);
      out.set(`${id} ${nth.toString()}`, element);
    }
  }
  return out;
}

/** assetOf is the id of the attachment an address of its content names (/api/v0/assets/{id}/content), if it does. */
function assetOf(address: string | null): string | undefined {
  return address === null ? undefined : /^\/api\/v0\/assets\/([^/?#]+)\/content(?:[?#]|$)/.exec(address)?.[1];
}

/**
 * signAnew gives element the address of its attachment signed anew, and has it go on where it was, at its rate,
 * playing if it was, its frame shown if not; nothing when the address is not given (the attachment gone) or the
 * element is no longer in the page.
 */
async function signAnew(element: HTMLMediaElement, { assetAddress }: ReadingContext): Promise<void> {
  const id = assetOf(element.getAttribute("src"));
  if (id === undefined) {
    return;
  }
  // As it failed: the new address's load has it paused, at the start, at the default rate.
  const at = element.currentTime;
  const playing = !element.paused;
  const rate = element.playbackRate;
  let address: string;
  try {
    address = await assetAddress(id);
  } catch {
    return;
  }
  if (!element.isConnected) {
    return;
  }
  try {
    if (!playing) {
      element.preload = "metadata";
    }
    element.setAttribute("src", address);
    element.playbackRate = rate;
    element.currentTime = at;
    // An engine that takes no position before the metadata takes it then.
    element.addEventListener(
      "loadedmetadata",
      () => {
        element.currentTime = at;
      },
      { once: true }
    );
    if (playing) {
      await element.play();
    }
  } catch {
    // Refused, the media stays as it is: the reader plays it again.
  }
}
