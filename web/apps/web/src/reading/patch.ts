import { begun } from "./assets";

/**
 * The reading view's HTML changed in place (M12/P2/S4; M12/P2 design
 * 4.4.2): what the server's HTML read again changes of the view's, the
 * rest left as it is, its nodes the same.
 *
 * An attachment's content's address (asset/adapter/http's ContentURL)
 * is signed until the end of the hour after it is read: a view read again
 * in another hour differs in its addresses' expiry (e) and signature (s)
 * alone. Another file (b), a download (d=1) or another attachment is a
 * change of the view. An address written in the text is text: only the
 * src and href of an element are addresses.
 */

/** address is an attachment's content's address as the server spells it: its expiry and signature what changes. */
const address =
  /^(\/api\/v0\/assets\/[0-9a-f-]{36}\/content\?b=[0-9a-f-]{36}&e=)[1-9][0-9]*(&s=)[A-Za-z0-9_-]{22}(&d=1)?$/;

/** The attributes an address is in. */
const addressed = ["src", "href"] as const;

/** The elements whose src or href may be an address: an attachment's content's, or another of the asset routes. */
const addressing = '[src^="/api/v0/assets/"], [href^="/api/v0/assets/"]';

/** assetOf is the attachment's id an element's address names, by its src or its href. */
function assetOf(element: Element): string | undefined {
  const value = element.getAttribute("src") ?? element.getAttribute("href") ?? "";
  return /^\/api\/v0\/assets\/([^/?#]+)\//.exec(value)?.[1];
}

/** template is html parsed, as the view writes it: its content is the view's nodes. */
export function template(html: string): HTMLTemplateElement {
  const parsed = document.createElement("template");
  parsed.innerHTML = html;
  return parsed;
}

/** maskAddresses has each address of root and the elements in it, in src or href, say no expiry and no signature. */
function maskAddresses(root: Element | DocumentFragment): void {
  const elements = root.querySelectorAll("[src], [href]");
  for (const element of root instanceof Element ? [root, ...elements] : elements) {
    for (const name of addressed) {
      const value = element.getAttribute(name);
      if (value !== null && address.test(value)) {
        element.setAttribute(name, value.replace(address, "$1$2$3"));
      }
    }
  }
}

/** masked is template's HTML, its addresses masked. */
function masked(parsed: HTMLTemplateElement): string {
  const copy = parsed.cloneNode(true) as HTMLTemplateElement;
  maskAddresses(copy.content);
  return copy.innerHTML;
}

/**
 * addressesOnly tells whether after is before but for its attachments' addresses' expiry and signature, signed anew
 * (the same HTML too): the view's nodes may stay, their addresses replaced (resign).
 */
export function addressesOnly(before: HTMLTemplateElement, after: HTMLTemplateElement): boolean {
  return masked(before) === masked(after);
}

/**
 * resign gives the elements of article, which has before's HTML, enhanced, the addresses of after's, whose
 * addresses alone differ (addressesOnly): a link its href, an image its src (the old one shown until the new one
 * loads, the node the same), an audio or a video not begun its src. One begun keeps its own: it goes on, signed anew
 * as it fails (assets.ts). No node is taken out or put in: the selection, the callouts opened, the focus and the
 * fullscreen stay (M12/P2/S4, 17.1, 17.8).
 *
 * The enhancements put in or take out no element with an address: article's are before's, the same many, each of
 * the same attachment in order. Not so, nothing is changed, and it answers false: the HTML is written as for a
 * change (U4).
 */
export function resign(article: Element, before: HTMLTemplateElement, after: HTMLTemplateElement): boolean {
  const old = [...before.content.querySelectorAll(addressing)];
  const next = [...after.content.querySelectorAll(addressing)];
  const live = [...article.querySelectorAll(addressing)];
  const alike = (element: Element, i: number) =>
    element.nodeName === next[i]?.nodeName && assetOf(element) === assetOf(next[i]);
  if (old.length !== next.length || !live.every(alike) || live.length !== next.length) {
    return false;
  }
  live.forEach((element, i) => {
    const signed = next[i];
    if (signed === undefined || (element instanceof HTMLMediaElement && begun(element))) {
      return;
    }
    for (const name of addressed) {
      const value = signed.getAttribute(name);
      if (value !== null && element.getAttribute(name) !== value) {
        element.setAttribute(name, value);
      }
    }
  });
  return true;
}

/**
 * Placed maps the place of an article's top-level node before a patch to its place after, or none. A patch's tells
 * too, of one taken out, the places of the other nodes put in where it was (its gap: between the kept nodes on each
 * side), as far as its new version may be, nearest first, the later of two as near first (near).
 */
export type Placed = ((at: number) => number | undefined) & { near?: (at: number) => number[] };

/**
 * patchBlocks has article, which has before's HTML with its enhancements undone, after's (U4; M12/P2/S4, 4.4.2):
 * top-level node by node, keeping those that did not change. Each is known by its HTML, its addresses' expiry and
 * signature and its ids left out (a heading renumbered is no change). The ends alike are kept, the end's first:
 * a section of the same name put before one keeps the one on its nodes, renumbered. Between them, each of after's
 * takes the first of before's of its HTML past the last taken (P2-21): a link's target made changes links far apart,
 * the blocks between kept; a blank text is no block. A kept node gets the ids and addresses after has, as before
 * had them: what the reader changed (a callout opened, an anchor's tabindex) stays, and so does a media begun. The
 * others are taken out, and after's put in their places; no node kept is moved, which would take it out a moment
 * (the focus lost, a video out of fullscreen).
 *
 * It answers where each of before's nodes is now: a node taken out is where it was, at the one put in its place
 * (the k-th taken out between two kept at the k-th put in, the last of them if fewer), else at the next kept, else
 * at the last block. Article's top-level nodes not before's (an enhancement not undone), nothing is changed and it
 * answers undefined: the HTML is written whole.
 */
export function patchBlocks(
  article: Element,
  before: HTMLTemplateElement,
  after: HTMLTemplateElement
): Placed | undefined {
  const live = [...article.childNodes];
  const old = [...before.content.childNodes];
  const next = [...after.content.childNodes];
  if (live.length !== old.length || live.some((node, i) => node.nodeName !== old[i]?.nodeName)) {
    return undefined;
  }
  const oldKeys = old.map(blockKey);
  const nextKeys = next.map(blockKey);
  let tail = 0;
  while (tail < old.length && tail < next.length && oldKeys.at(-1 - tail) === nextKeys.at(-1 - tail)) {
    tail += 1;
  }
  let head = 0;
  while (head < old.length - tail && head < next.length - tail && oldKeys[head] === nextKeys[head]) {
    head += 1;
  }
  const pairs: [number, number][] = [];
  for (let i = 0; i < head; i++) {
    pairs.push([i, i]);
  }
  pairs.push(...between(old, oldKeys, next, nextKeys, head, tail));
  for (let k = tail; k > 0; k--) {
    pairs.push([old.length - k, next.length - k]);
  }
  // A kept node whose ids or addresses changed, and whose nodes are not before's (code coloured), is put in anew.
  const kept = new Map(pairs.filter(([i, j]) => restamp(live[i], old[i], next[j])));
  const at = new Map([...kept].map(([i, j]) => [j, i]));
  for (const [i, node] of live.entries()) {
    if (!kept.has(i)) {
      node.remove();
    }
  }
  let following: Node | null = null;
  for (let j = next.length - 1; j >= 0; j--) {
    const i = at.get(j);
    const node = i === undefined ? article.ownerDocument.importNode(next[j] as Node, true) : (live[i] as Node);
    if (i === undefined) {
      article.insertBefore(node, following);
    }
    following = node;
  }
  return Object.assign((place: number) => placed(place, kept, old, next), {
    near: (place: number) => near(place, kept, old, next),
  });
}

/**
 * between pairs the nodes of old and next between their ends alike (head and tail of them): each of next's takes the
 * first of old's of its key past the last taken. The pairs go up in both.
 */
function between(
  old: Node[],
  oldKeys: string[],
  next: Node[],
  nextKeys: string[],
  head: number,
  tail: number
): [number, number][] {
  // Old's places of each key, going up, and how many of them are taken or passed: each goes in and out once.
  const waiting = new Map<string, { places: number[]; from: number }>();
  for (let i = head; i < old.length - tail; i++) {
    if (!blank(old[i])) {
      const key = oldKeys[i] ?? "";
      const queue = waiting.get(key) ?? { places: [], from: 0 };
      queue.places.push(i);
      waiting.set(key, queue);
    }
  }
  const pairs: [number, number][] = [];
  let last = head - 1;
  for (let j = head; j < next.length - tail; j++) {
    const queue = blank(next[j]) ? undefined : waiting.get(nextKeys[j] ?? "");
    if (queue === undefined) {
      continue;
    }
    while (queue.from < queue.places.length && (queue.places[queue.from] ?? 0) <= last) {
      queue.from += 1;
    }
    const i = queue.places[queue.from];
    if (i !== undefined) {
      queue.from += 1;
      pairs.push([i, j]);
      last = i;
    }
  }
  return pairs;
}

/** placed is where old's node at place is in next, kept being where the kept are. */
function placed(place: number, kept: Map<number, number>, old: Node[], next: Node[]): number | undefined {
  const now = kept.get(place);
  if (now !== undefined) {
    return now;
  }
  const { removed, inserted, following } = gap(place, kept, old, next);
  if (inserted.length > 0) {
    return inserted[Math.min(Math.max(removed.indexOf(place), 0), inserted.length - 1)];
  }
  return following.find(([, j]) => !blank(next[j]))?.[1] ?? range(0, next.length).findLast((j) => !blank(next[j]));
}

/**
 * near are the places of the nodes put in where the node at place was taken out, but the one it is placed at, nearest
 * to that one first, the later of two as near first: a section put before the reader's, which another changed in the
 * same save, has the reader's after it (review A's C1-M1). Only as far as the node's new version may be: placed k-th
 * for k-th, it is off by as many nodes as were put in or taken out there, one at least; a gap a save changed whole,
 * as many put in as taken out, reaches far, where a node of the key would be another (review A's C2-M1). Counted in
 * nodes, blank text left out. None for a node kept, or a gap nothing was put in.
 */
function near(place: number, kept: Map<number, number>, old: Node[], next: Node[]): number[] {
  const to = placed(place, kept, old, next);
  if (kept.has(place) || to === undefined) {
    return [];
  }
  const { removed, inserted } = gap(place, kept, old, next);
  const at = inserted.indexOf(to);
  const reach = Math.max(1, Math.abs(inserted.length - removed.length));
  return inserted
    .map((j, k) => ({ j, k, far: Math.abs(k - at) }))
    .filter(({ far }) => at >= 0 && far > 0 && far <= reach)
    .toSorted((a, b) => a.far - b.far || b.k - a.k)
    .map(({ j }) => j);
}

/**
 * gap is where the node at place, taken out, was: between the kept nodes on each side, the nodes taken out (old's
 * places) and put in (next's), blank text left out, and the kept nodes that follow.
 */
function gap(place: number, kept: Map<number, number>, old: Node[], next: Node[]) {
  // The pairs go up in both: kept has them in order.
  const sorted = [...kept];
  const previous = sorted.findLast(([i]) => i < place);
  const following = sorted.filter(([i]) => i > place);
  const end = following[0];
  const removed = range(previous === undefined ? 0 : previous[0] + 1, end?.[0] ?? old.length).filter(
    (i) => !blank(old[i])
  );
  const inserted = range(previous === undefined ? 0 : previous[1] + 1, end?.[1] ?? next.length).filter(
    (j) => !blank(next[j])
  );
  return { removed, inserted, following };
}

/** range is from up to to, not with it. */
function range(from: number, to: number): number[] {
  return Array.from({ length: Math.max(to - from, 0) }, (_, k) => from + k);
}

/** blank tells whether node is a text of white space, which matches nothing: it goes with the blocks around it. */
function blank(node: Node | undefined): boolean {
  return node?.nodeType === Node.TEXT_NODE && (node.textContent ?? "").trim() === "";
}

/** blockKey is what a top-level node is: an element's HTML, its addresses masked and its ids blank; a text's text. */
function blockKey(node: Node): string {
  if (!(node instanceof Element)) {
    return `${node.nodeName} ${node.nodeValue ?? ""}`;
  }
  const copy = node.cloneNode(true) as Element;
  maskAddresses(copy);
  for (const element of [copy, ...copy.querySelectorAll("[id]")]) {
    if (element.hasAttribute("id")) {
      element.setAttribute("id", "");
    }
  }
  return copy.outerHTML;
}

/** The attributes a kept node takes from the HTML read again: those its key leaves out. */
const stamps = ["id", ...addressed] as const;

/**
 * restamp gives live, a kept node that is old with its enhancements undone, the ids and addresses next has in place of
 * old's: from old to next, not from live to next, so what the reader changed stays, and a media begun keeps its
 * address. It tells whether live is kept: not when they changed and its nodes are not old's. A code block's code is
 * left out on each side: coloured, it has nodes of its own, and none has an id or an address (review A's M2).
 */
function restamp(live: Node | undefined, old: Node | undefined, next: Node | undefined): boolean {
  if (!(live instanceof Element && old instanceof Element && next instanceof Element)) {
    return true;
  }
  const olds = stamped(old);
  const nexts = stamped(next);
  const changed = olds.flatMap((element, k) =>
    stamps.filter((name) => element.getAttribute(name) !== nexts[k]?.getAttribute(name)).map((name) => ({ k, name }))
  );
  if (changed.length === 0) {
    return true;
  }
  const lives = stamped(live);
  if (lives.length !== olds.length || lives.some((element, k) => element.nodeName !== olds[k]?.nodeName)) {
    return false;
  }
  for (const { k, name } of changed) {
    const element = lives[k];
    const value = nexts[k]?.getAttribute(name) ?? null;
    if (element === undefined || (name === "src" && element instanceof HTMLMediaElement && begun(element))) {
      continue;
    }
    if (value === null) {
      element.removeAttribute(name);
    } else {
      element.setAttribute(name, value);
    }
  }
  return true;
}

/** stamped are root and the elements in it, in order, but those in a code block's code (pre > code). */
function stamped(root: Element): Element[] {
  return [root, ...root.querySelectorAll("*")].filter(
    (element) => element === root || element.parentElement?.closest("pre > code") === null
  );
}

/** FocusKey is how an element that had the focus is found in the view written anew: what it is, and which of them. */
export type FocusKey = { key: string; nth: number };

/** The elements a focus key may be of. */
const keyed = "h1, h2, h3, h4, h5, h6, a, summary, audio, video, [id]";

/** text is element's text, its white space collapsed. */
function text(element: Element): string {
  return (element.textContent ?? "").replace(/\s+/g, " ").trim();
}

/**
 * keyOf is what element is, to find it again (M12/P2/S4, 4.4.3's step 3): a heading its level and text, not its id,
 * which a heading of the same name put before changes; a link its address (an attachment's masked) and its text, one
 * to no page its target; a callout's summary its text; an audio or a video its attachment; another element its id.
 * None, it has no key.
 */
function keyOf(element: Element): string | undefined {
  const name = element.localName;
  if (/^h[1-6]$/.test(name)) {
    return `${name} ${text(element)}`;
  }
  if (name === "a") {
    const href = element.getAttribute("href");
    return href === null
      ? `a to ${element.getAttribute("data-nw-target") ?? ""} ${text(element)}`
      : `a ${href.replace(address, "$1$2$3")} ${text(element)}`;
  }
  if (name === "summary") {
    return `summary ${text(element)}`;
  }
  if (name === "audio" || name === "video") {
    const asset = assetOf(element);
    return asset === undefined ? undefined : `media ${asset}`;
  }
  return element.id === "" ? undefined : `#${element.id}`;
}

/**
 * focusKeyOf is element's key in within, and which of within's of that key it is, within among them; none for one
 * without a key. Within is the top-level node it is in (M12/P2/S4, 4.4.3's step 3, review A's M4): found again in
 * the block it maps to, not a link of the same address far off, which the view would scroll to.
 */
export function focusKeyOf(within: Element, element: Element): FocusKey | undefined {
  const key = keyOf(element);
  if (key === undefined) {
    return undefined;
  }
  return { key, nth: ofKey(within, key).indexOf(element) };
}

/** findByKey is within's element of key, the nth of them, within among them; none if there is not one. */
export function findByKey(within: Element, { key, nth }: FocusKey): HTMLElement | undefined {
  const found = ofKey(within, key)[nth];
  return found instanceof HTMLElement ? found : undefined;
}

/** ofKey are within and the elements in it of key, in order. */
function ofKey(within: Element, key: string): Element[] {
  return [within, ...within.querySelectorAll(keyed)].filter((each) => each.matches(keyed) && keyOf(each) === key);
}
