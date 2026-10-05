import type { Enhancement, UnresolvedLink } from "./enhancement";

/**
 * unresolvedLinks lets a reader act on the links to pages that are not
 * there (nw-unresolved, M6/P6 design 7), which have no address: each is a
 * button that opens a dialog, focusable, and acted on by a click, Enter or
 * Space, which hands it to the view (ReadingContext.unresolved) with what
 * it is: an embed's (nw-embed) or a Markdown image's link only says it is
 * not there, a link may have its page created. The view decides which,
 * by the reader's role.
 */
export const unresolvedLinks: Enhancement = (container, { unresolved }) => {
  const links = [...container.querySelectorAll<HTMLAnchorElement>("a.nw-unresolved[data-nw-target]")];
  if (links.length === 0) {
    return undefined;
  }
  for (const link of links) {
    link.setAttribute("role", "button");
    link.setAttribute("tabindex", "0");
    link.setAttribute("aria-haspopup", "dialog");
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
  const onKey = (event: KeyboardEvent) => {
    const plain = !event.altKey && !event.ctrlKey && !event.metaKey && !event.shiftKey;
    if (!event.defaultPrevented && plain && !event.isComposing && (event.key === "Enter" || event.key === " ")) {
      act(event);
    }
  };
  container.addEventListener("click", onClick);
  container.addEventListener("keydown", onKey);
  return () => {
    container.removeEventListener("click", onClick);
    container.removeEventListener("keydown", onKey);
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
