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
 * - A link to one the browser shows opens it in a tab of its own, as it
 *   says unseen; one to any other downloads it (the server writes
 *   download), and stays as it is.
 * - An image, an audio or a video that fails to load, once the view's
 *   addresses have expired, reads the view again: once for each expiry of
 *   the page's, as one that fails for another reason would fail as much
 *   in the view read again.
 * - An audio or a video started (playing, or paused partway) is kept as
 *   the HTML is replaced, which the signatures do each hour and others'
 *   writes at any time: the next HTML of the page has it in place of its
 *   own of the same attachment, the same of them in order, its attributes
 *   but its address; one it does not have goes. Its address stays the
 *   one it had: as that fails, once it has expired, the view has the
 *   attachment's signed anew (assetAddress), and the media goes on where
 *   it was; once for each HTML, none for an attachment gone.
 */
export function assets(): Enhancement {
  // What the view had started as its HTML was replaced: its page, and its audios and videos by key.
  let kept: { page: string; media: Map<string, HTMLMediaElement> } | undefined;
  // The expiry each page's view was read again for, as its attachments failed to load.
  const reloadedFor = new Map<string, string>();
  return (container, context) => {
    const adopted = adopt(container, kept?.page === context.page ? kept.media : undefined);
    kept = undefined;
    const added = newTab(container, context.t("asset.newTab"));
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
      for (const { link, hint } of added) {
        link.removeAttribute("target");
        link.removeAttribute("rel");
        for (const node of hint) {
          node.remove();
        }
      }
      kept = { page: context.page, media: started(container) };
    };
  };
}

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
 * adopt puts each of kept, by key, in place of the element of container
 * of the same key and element, its attributes but its address taken;
 * those put are the adopted.
 */
function adopt(container: HTMLElement, kept: Map<string, HTMLMediaElement> | undefined): Set<HTMLMediaElement> {
  const adopted = new Set<HTMLMediaElement>();
  if (kept === undefined) {
    return adopted;
  }
  for (const [key, element] of keyed(container)) {
    const old = kept.get(key);
    if (old === undefined || old.localName !== element.localName) {
      continue;
    }
    for (const name of old.getAttributeNames()) {
      if (name !== "src") {
        old.removeAttribute(name);
      }
    }
    for (const { name, value } of element.attributes) {
      if (name !== "src") {
        old.setAttribute(name, value);
      }
    }
    element.replaceWith(old);
    adopted.add(old);
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
 * signAnew gives element the address of its attachment signed anew, and has it go on where it was, playing if it
 * was; nothing when the address is not given (the attachment gone) or the element is no longer in the page.
 */
async function signAnew(element: HTMLMediaElement, { assetAddress }: ReadingContext): Promise<void> {
  const id = assetOf(element.getAttribute("src"));
  if (id === undefined) {
    return;
  }
  const at = element.currentTime;
  const playing = !element.paused;
  let address: string;
  try {
    address = await assetAddress(id);
  } catch {
    return;
  }
  if (!element.isConnected) {
    return;
  }
  element.setAttribute("src", address);
  element.currentTime = at;
  if (playing) {
    await element.play().catch(() => undefined);
  }
}
