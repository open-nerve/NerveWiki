import type { Enhancement, UnresolvedLink } from "./enhancement";

/**
 * unresolvedLinks lets a reader act on the links to pages that are not
 * there (nw-unresolved, M6/P6 design 7), which have no address: each is a
 * button that opens a dialog, focusable, and acted on by a click, Enter or
 * Space, which hands it to the view (ReadingContext.unresolved) with what
 * it is: an embed's (nw-embed) or a Markdown image's link only says it is
 * not there, a link may have its page created. The view decides which,
 * by the reader's role. Enter acts as it goes down, once however long it
 * is held, Space as it comes up on the link it went down on, as on a
 * button. One the view goes to a page by (leads: an LLM Wiki's directory
 * or history) is a link, which opens no dialog, and Space is the page's
 * (M12/P2/S1). A diagram's links are mermaid's, not the server's: they
 * are left be (diagrams.ts).
 */
export const unresolvedLinks: Enhancement = (container, { unresolved, leads }) => {
  const links = [...container.querySelectorAll<HTMLAnchorElement>("a.nw-unresolved[data-nw-target]")].filter(
    (link) => link.closest(".nw-diagram") === null
  );
  if (links.length === 0) {
    return undefined;
  }
  // Those that open a dialog.
  const buttons = new Set<HTMLAnchorElement>();
  for (const link of links) {
    // An embed's or an image's link only says it is not there, whatever its target.
    const goes = kindOf(link) === "link" && leads?.(link.dataset.nwTarget ?? "") === true;
    link.setAttribute("role", goes ? "link" : "button");
    link.setAttribute("tabindex", "0");
    if (!goes) {
      link.setAttribute("aria-haspopup", "dialog");
      buttons.add(link);
    }
  }
  const act = (event: Event) => {
    const link = event.target instanceof Element ? event.target.closest("a") : null;
    if (link === null || !links.includes(link)) {
      return;
    }
    event.preventDefault();
    unresolved({ target: link.dataset.nwTarget ?? "", kind: kindOf(link), element: link });
  };
  const onClick = (event: MouseEvent) => {
    if (!event.defaultPrevented && event.button === 0) {
      act(event);
    }
  };
  // The link Space went down on: it acts as it comes up there, as on a button, not on another the focus went to.
  let spaced: HTMLAnchorElement | undefined;
  const onKey = (event: KeyboardEvent) => {
    const plain = !event.altKey && !event.ctrlKey && !event.metaKey && !event.shiftKey;
    if (event.defaultPrevented || !plain || event.isComposing) {
      return;
    }
    // Held down, Enter repeats: the dialog it opened would open again (M12/P2/S1, 11.7).
    if (event.type === "keydown" && event.key === "Enter" && !event.repeat) {
      act(event);
    } else if (event.key === " ") {
      const link = event.target instanceof Element ? event.target.closest("a") : null;
      if (event.type === "keydown") {
        if (link !== null && buttons.has(link)) {
          // Down, Space would scroll the page.
          event.preventDefault();
          spaced = link;
        }
      } else {
        const down = spaced;
        spaced = undefined;
        if (link !== null && link === down) {
          act(event);
        }
      }
    }
  };
  container.addEventListener("click", onClick);
  container.addEventListener("keydown", onKey);
  container.addEventListener("keyup", onKey);
  return () => {
    container.removeEventListener("click", onClick);
    container.removeEventListener("keydown", onKey);
    container.removeEventListener("keyup", onKey);
    for (const link of links) {
      link.removeAttribute("role");
      link.removeAttribute("tabindex");
      link.removeAttribute("aria-haspopup");
    }
  };
};

/** kindOf is what link is: an embed's, a Markdown image's (in its span), or a link. */
function kindOf(link: HTMLAnchorElement): UnresolvedLink["kind"] {
  if (link.classList.contains("nw-embed")) {
    return "embed";
  }
  return link.closest(".nw-image") === null ? "link" : "image";
}
