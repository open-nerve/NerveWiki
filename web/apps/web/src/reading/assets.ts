import { formatBytes } from "../i18n/format";
import type { Enhancement, ReadingContext } from "./enhancement";
import { unfold } from "./unfold";

/** How many audios and videos a view keeps playing as its HTML is replaced: as many as it writes (obsidian's MaxMedia). */
const maxKept = 20;

/** The attachments' audios and videos a view writes. */
const media = "audio.nw-asset, video.nw-asset";

/** How far from where it was a media may start, in seconds, without being moved there again. */
const nearEnough = 0.25;

/** How long after a media's address was signed anew a failure of it is its file's, not its address's: it is not signed again. */
const signedAgainAfter = 60_000;

/** How long after a media's signing anew was refused it is signed again: at first; at most, twice as long each time. */
const refusedRetry = 30_000;
const refusedRetryMax = 10 * 60_000;

/**
 * assets is the attachments of the reading view (M7/P4 design 4.4–4.6),
 * whose HTML has them at their contents' addresses, signed for an hour or
 * more:
 *
 * - A link to one says its size after it, in the reader's language. One
 *   the browser shows opens in a tab of its own, as it says unseen; one to
 *   any other downloads it (the server writes download).
 * - An audio or a video started (playing, paused partway, or played and
 *   not loaded yet) that fails (its address expired, as a rule: a long
 *   one, a tab hidden, a reader away) has the attachment's address signed
 *   anew (assetAddress), and goes on where it was, the same element; not
 *   while it is being signed anew, nor within signedAgainAfter of it (it
 *   fails for another reason then).
 * - An image, or an audio or a video not signed anew, that fails to load,
 *   or an address not given, once the view's addresses have expired reads
 *   the view again: once for each expiry of the page's, as one that fails
 *   for another reason would fail as much in the view read again. One
 *   that fails while it is being signed anew is let be.
 * - An audio or a video started is kept as the HTML is replaced, which
 *   the signatures do each hour and others' writes at any time: the next
 *   HTML of the page has it in place of its own of the same attachment,
 *   the same of them in order, its attributes but its address, the focus
 *   if it had it (a folded callout it is in opening); one it does not
 *   have goes. One that failed is not kept: it would not load again, and
 *   the new one is put where it was (its position, rate and volume, the
 *   focus), not played: it failed signed anew too, or could not be; but
 *   for one being signed anew as it played, whose new one plays on.
 * - An audio or a video whose signing anew was refused (the network, as a
 *   rule), the view not expired, is signed anew again from where it
 *   failed (its position and rate): as the browser is online again; half
 *   a minute later, then twice as long each time up to ten minutes, the
 *   view run again starting its timer again; and as it fails again, its
 *   controls having loaded it anew (Chromium's), started or not. Not one
 *   whose old address plays again, past where it failed or moved there by
 *   the reader: it is refused no more. The HTML replaced meanwhile puts its
 *   new one there (M12/P2/S5, 17.7; review A's check 2).
 */
export function assets(): Enhancement {
  // What the view had started as its HTML was replaced: its page, its audios and videos by key, and the focused one.
  let kept: Kept | undefined;
  // The expiry each page's view was read again for, as its attachments failed to load.
  const reloadedFor = new Map<string, string>();
  const signing: Signing = { pending: new WeakMap(), at: new WeakMap(), refused: new Map() };
  return (container, context) => {
    adopt(container, kept?.page === context.page ? kept : undefined, signing);
    kept = undefined;
    // Those refused that the HTML replaced, and adopt did not put back, are gone.
    for (const element of signing.refused.keys()) {
      if (!element.isConnected) {
        forget(element, signing);
      }
    }
    const links = newTab(container, context.t("asset.newTab"));
    const added = [...links.flatMap(({ hint }) => hint), ...sizes(container, context)];
    const readAgain = () => {
      const expires = context.assetsExpire;
      if (expires !== null && Date.now() >= Date.parse(expires) && reloadedFor.get(context.page) !== expires) {
        reloadedFor.set(context.page, expires);
        context.reload();
      }
    };
    const onError = (event: Event) => {
      const element = event.target;
      if (!(element instanceof HTMLElement) || !element.matches(`img.nw-asset, ${media}`)) {
        return;
      }
      const failing = element instanceof HTMLMediaElement ? signing.pending.get(element) : undefined;
      if (failing !== undefined) {
        // A load of its old address meanwhile, which lost its place: the new one comes.
        failing.reloaded = true;
        return;
      }
      // Refused, it is signed anew from where it failed, its controls having loaded it anew or not.
      if (element instanceof HTMLMediaElement && signing.refused.has(element)) {
        void signAnew(element, context, signing, readAgain);
        return;
      }
      if (
        element instanceof HTMLMediaElement &&
        begun(element) &&
        Date.now() - (signing.at.get(element) ?? Number.NEGATIVE_INFINITY) >= signedAgainAfter
      ) {
        void signAnew(element, context, signing, readAgain);
        return;
      }
      readAgain();
    };
    // An element's error goes no further than it: the container hears it on its way down.
    container.addEventListener("error", onError, true);
    // One refused the reader moves, its error gone, has its old address playing again: it is refused no more. One
    // still broken can be moved too, in what it had (Firefox keeps that on a network error): it is still refused.
    const onSeeking = (event: Event) => {
      const element = event.target;
      if (element instanceof HTMLMediaElement && element.error === null && !signing.pending.has(element)) {
        forget(element, signing);
      }
    };
    container.addEventListener("seeking", onSeeking, true);
    // Online again, those refused are signed anew.
    const online = () => {
      for (const element of signing.refused.keys()) {
        if (container.contains(element)) {
          signAgain(element, context, signing, readAgain);
        }
      }
    };
    window.addEventListener("online", online);
    // Those refused this view had, left where they were as it runs again, have their timers again: the undo cleared them.
    for (const [element, refused] of signing.refused) {
      if (container.contains(element)) {
        retryLater(element, refused, () => signAgain(element, context, signing, readAgain));
      }
    }
    return () => {
      container.removeEventListener("error", onError, true);
      container.removeEventListener("seeking", onSeeking, true);
      window.removeEventListener("online", online);
      // Their timers go with the view; what they failed at stays, for an element kept and for the HTML next.
      for (const [element, refused] of signing.refused) {
        if (container.contains(element)) {
          clearTimeout(refused.retry);
          refused.retry = undefined;
        }
      }
      for (const { link } of links) {
        link.removeAttribute("target");
        link.removeAttribute("rel");
      }
      for (const node of added) {
        node.remove();
      }
      const playing = started(container, signing.refused);
      const active = document.activeElement;
      kept = {
        page: context.page,
        media: playing,
        focused: [...playing.values()].find((element) => element === active),
        had: new WeakSet(container.querySelectorAll<HTMLMediaElement>(media)),
      };
    };
  };
}

/**
 * Kept is what a view had started as its HTML was replaced: its page, its audios and videos by key, the focused one;
 * and every audio and video it had, started or not, which the next run leaves where it is if still there.
 */
type Kept = {
  page: string;
  media: Map<string, HTMLMediaElement>;
  focused: HTMLMediaElement | undefined;
  had: WeakSet<HTMLMediaElement>;
};

/**
 * Signing is the audios and videos signed anew: those under way, as each failed, and when each was last; and those
 * refused, where each failed and when it is signed again.
 */
type Signing = {
  pending: WeakMap<HTMLMediaElement, Failing>;
  at: WeakMap<HTMLMediaElement, number>;
  refused: Map<HTMLMediaElement, Refused>;
};

/** Refused is where an audio or a video whose signing anew was refused failed, and its next try: when, and how long after. */
type Refused = { place: Place; retry: ReturnType<typeof setTimeout> | undefined; after: number };

/** Failing is where an audio or a video being signed anew failed, and whether it was loaded again since. */
type Failing = { place: Place; reloaded: boolean };

/**
 * newTab has the links of container to attachments the browser shows open in a tab of their own, saying so unseen:
 * each link, and its hint, the space before it outside, which a name joins the words by. The hint is not copied with
 * the text (select-none; M12/P2/S5, 17.2b).
 */
function newTab(container: HTMLElement, text: string): { link: HTMLAnchorElement; hint: ChildNode[] }[] {
  const added: { link: HTMLAnchorElement; hint: ChildNode[] }[] = [];
  for (const link of container.querySelectorAll<HTMLAnchorElement>("a.nw-asset[href]:not([download])")) {
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    const hint = document.createElement("span");
    hint.className = "sr-only select-none";
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
 * it. One that failed is not put: the new one is put where it was, the
 * focus given it if it had it, playing if the one was as it was being
 * signed anew. One the view had that is still in container is where it
 * was, its block kept as the HTML changed (M12/P2/S4, 17.8): it is not
 * moved, which would take it out of the document a moment (and a video
 * out of fullscreen), nor taken by another of its attachment's put before
 * it, nor put in place of by another started whose block changed (review
 * A's M1, C's M1). The keys count every audio and video of the attachment,
 * so one put before a kept one, in a block changed, is not taken back:
 * it stops (the limit 4.4.8 writes).
 */
function adopt(container: HTMLElement, kept: Kept | undefined, signing: Signing) {
  if (kept === undefined) {
    return;
  }
  for (const [key, element] of keyed(container)) {
    const old = kept.media.get(key);
    if (old === undefined || container.contains(old) || kept.had.has(element)) {
      continue;
    }
    if (old.error !== null) {
      const failing = signing.pending.get(old);
      // One refused, where it failed, its controls having lost it since.
      putAt(element, signing.refused.get(old)?.place ?? goingOn(old, failing));
      signing.refused.delete(old);
      if (failing !== undefined && !old.paused) {
        void playOn(element);
      }
      if (old === kept.focused) {
        focus(element);
      }
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
    if (old === kept.focused) {
      focus(old);
    }
  }
}

/** focus gives element the focus, without a scroll; a folded callout it is in opens first, as it takes none closed. */
function focus(element: HTMLMediaElement) {
  unfold(element);
  element.focus({ preventScroll: true });
}

/**
 * started are the audios and videos of container started and not ended, or refused (whose new ones are put where they
 * failed: review A's check 2, M2), by key: the first maxKept of them.
 */
function started(container: HTMLElement, refused: Map<HTMLMediaElement, Refused>): Map<string, HTMLMediaElement> {
  const out = new Map<string, HTMLMediaElement>();
  for (const [key, element] of keyed(container)) {
    if (out.size < maxKept && (begun(element) || refused.has(element))) {
      out.set(key, element);
    }
  }
  return out;
}

/** begun tells whether element was started, playing or paused partway, and has not ended. */
export function begun(element: HTMLMediaElement): boolean {
  return (!element.paused || element.currentTime > 0) && !element.ended;
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
export function assetOf(address: string | null): string | undefined {
  return address === null ? undefined : /^\/api\/v0\/assets\/([^/?#]+)\/content(?:[?#]|$)/.exec(address)?.[1];
}

/**
 * signAnew gives element the address of its attachment signed anew, and has it go on where it is, as the reader left
 * it; or, when a load of it meanwhile (Chromium's controls, as one plays it) lost them, at the position and rate it
 * failed at. Nothing when the element is no longer in the page; refused, as the address is not given
 * (the attachment gone, or the network), and signed anew again later (noteRefused). One refused before is signed from
 * where it failed then.
 */
async function signAnew(
  element: HTMLMediaElement,
  context: ReadingContext,
  signing: Signing,
  refused: () => void
): Promise<void> {
  const { assetAddress } = context;
  const id = assetOf(element.getAttribute("src"));
  if (id === undefined) {
    return;
  }
  const before = signing.refused.get(element);
  const failing: Failing = { place: before?.place ?? placeOf(element), reloaded: before !== undefined };
  signing.pending.set(element, failing);
  let address: string;
  try {
    address = await assetAddress(id);
  } catch {
    if (element.isConnected) {
      noteRefused(element, failing.place, signing, () => signAgain(element, context, signing, refused));
      refused();
    }
    return;
  } finally {
    signing.pending.delete(element);
  }
  if (!element.isConnected) {
    return;
  }
  forget(element, signing);
  signing.at.set(element, Date.now());
  // Before the new address's load has it paused, at the start, at the default rate.
  const place = goingOn(element, failing);
  element.setAttribute("src", address);
  if (!place.playing) {
    element.preload = "metadata";
  }
  putAt(element, place);
  if (place.playing) {
    await playOn(element);
  }
}

/**
 * noteRefused notes element's signing anew refused, where it failed (the first place noted, while it is refused), and
 * has it signed again (signAgain) refusedRetry later, then twice as long each time, up to refusedRetryMax.
 */
function noteRefused(element: HTMLMediaElement, place: Place, signing: Signing, again: () => void) {
  const was = signing.refused.get(element);
  clearTimeout(was?.retry);
  const refused: Refused = {
    place: was?.place ?? place,
    retry: undefined,
    after: was === undefined ? refusedRetry : Math.min(was.after * 2, refusedRetryMax),
  };
  signing.refused.set(element, refused);
  retryLater(element, refused, again);
}

/** retryLater has element, refused, signed again (again) refused.after from now, while it is in the page. */
function retryLater(element: HTMLMediaElement, refused: Refused, again: () => void) {
  clearTimeout(refused.retry);
  refused.retry = setTimeout(() => {
    if (element.isConnected) {
      again();
    }
  }, refused.after);
}

/**
 * signAgain signs element anew again (signAnew), while it is refused and not being signed; but not as its old address
 * plays again, its error gone, at or past where it failed: the reader went on with it, whom it would pull back (review
 * A's check 2, M1). It is refused no more then. One before where it failed, its controls having loaded it anew at the
 * start, is signed.
 */
function signAgain(element: HTMLMediaElement, context: ReadingContext, signing: Signing, refused: () => void) {
  const was = signing.refused.get(element);
  if (was === undefined || signing.pending.has(element)) {
    return;
  }
  if (element.error === null && element.currentTime >= was.place.at) {
    forget(element, signing);
    return;
  }
  void signAnew(element, context, signing, refused);
}

/** forget has element refused no more: its timer cleared, where it failed forgotten. */
function forget(element: HTMLMediaElement, signing: Signing) {
  clearTimeout(signing.refused.get(element)?.retry);
  signing.refused.delete(element);
}

/** playOn plays element; refused, it stays where it is, which the reader plays again. */
async function playOn(element: HTMLMediaElement): Promise<void> {
  try {
    await element.play();
  } catch {
    // Refused.
  }
}

/** Place is where an audio or a video was: its position, its rate, its volume, whether muted, and whether it played. */
type Place = { at: number; rate: number; volume: number; muted: boolean; playing: boolean };

/**
 * goingOn is where element goes on from, as it is; but at the position and rate it failed at, being signed anew
 * (failing), when it was loaded again since, which lost them: its error cleared, or set again by the load's failure.
 */
function goingOn(element: HTMLMediaElement, failing: Failing | undefined): Place {
  const now = placeOf(element);
  return failing !== undefined && (failing.reloaded || element.error === null)
    ? { ...now, at: failing.place.at, rate: failing.place.rate }
    : now;
}

/** placeOf is where element is. */
function placeOf(element: HTMLMediaElement): Place {
  const { currentTime: at, playbackRate: rate, volume, muted, paused } = element;
  return { at, rate, volume, muted, playing: !paused };
}

/**
 * putAt has element, loading its address, at place: its position, its rate, its volume. An engine takes the position
 * before the metadata, or else as they come, and one that throws takes it then too.
 */
function putAt(element: HTMLMediaElement, { at, rate, volume, muted }: Place) {
  element.playbackRate = rate;
  element.volume = volume;
  element.muted = muted;
  element.addEventListener(
    "loadedmetadata",
    () => {
      if (Math.abs(element.currentTime - at) > nearEnough) {
        element.currentTime = at;
      }
    },
    { once: true }
  );
  try {
    element.currentTime = at;
  } catch {
    // Taken as the metadata come.
  }
}
