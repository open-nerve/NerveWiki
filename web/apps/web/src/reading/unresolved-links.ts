import type { Enhancement, UnresolvedLink } from "./enhancement";

/**
 * unresolvedLinks lets a reader act on the links to pages that are not
 * there (nw-unresolved, M6/P6 design 7), which have no address: each is a
 * button that opens a dialog, focusable, and acted on by a click, Enter or
 * Space, which hands it to the view (ReadingContext.unresolved) with what
 * it is: an embed's (nw-embed) or a Markdown image's link only says it is
 * not there, a link may have its page created. The view decides which,
 * by the reader's role. Enter acts as it goes down, Space as it comes up,
 * as on a button. A diagram's links are mermaid's, not the server's: they
 * are left be (diagrams.ts).
 */
export const unresolvedLinks: Enhancement = (container, { unresolved }) => {
  const links = [...container.querySelectorAll<HTMLAnchorElement>("a.nw-unresolved[data-nw-target]")].filter(
    (link) => link.closest(".nw-diagram") === null
  );
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
    if (event.defaultPrevented || !plain || event.isComposing) {
      return;
    }
    if (event.type === "keydown" && event.key === "Enter") {
      act(event);
    } else if (event.key === " ") {
      // Down, Space would scroll the page.
      const link = event.target instanceof Element ? event.target.closest("a") : null;
      if (event.type === "keyup") {
        act(event);
      } else if (link !== null && links.includes(link)) {
        event.preventDefault();
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
