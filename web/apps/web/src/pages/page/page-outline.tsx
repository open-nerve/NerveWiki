import { useMemo } from "react";
import { Link } from "react-router";

import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { usePageView } from "./page-view";
import { PanelSection } from "./panel-section";

/** A heading of the page: its id, its level and its text. */
type Heading = { id: string; level: number; text: string };

/** The outline reads the view only as the reading view does: on its own, never. */
const readsNothing = {
  revalidateOnMount: false,
  revalidateOnFocus: false,
  revalidateOnReconnect: false,
  revalidateIfStale: false,
} as const;

/**
 * PageOutline is the page's headings as its reading view has them (M6/P7
 * design 8): from the HTML the view read, under its key, with no read of
 * its own. Each is indented by its level, from the page's highest, and
 * leads to its heading through the router, as a link of the page to its
 * anchor does: the view has the heading show and take the focus. A page
 * without headings has no outline.
 */
export function PageOutline({ notebook, page }: { notebook: Notebook; page: string }) {
  const t = useT();
  const { data } = usePageView(notebook, page, readsNothing);
  const html = data?.html;
  const headings = useMemo(() => (html === undefined ? [] : headingsOf(html)), [html]);
  if (headings.length === 0) {
    return null;
  }
  const top = Math.min(...headings.map(({ level }) => level));
  return (
    <PanelSection title={t("page.outline")}>
      <nav aria-label={t("page.outline")}>
        <ul className="space-y-1 text-sm">
          {headings.map(({ id, level, text }) => (
            <li key={id} style={{ paddingLeft: `${(level - top) * 0.75}rem` }}>
              <Link to={{ hash: encodeURIComponent(id) }} className="block break-words hover:underline">
                {text}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </PanelSection>
  );
}

/**
 * headingsOf is the headings of a page's HTML that an anchor leads to: those
 * with an id the server gave (nw-), in their order, but those of the
 * footnotes, with their text: a formula's is its TeX, as the server writes
 * it; a footnote's number and an image's address are not. One without
 * text is left out, as it would be a link to nothing one could read. The
 * HTML, the server's sanitized, is parsed in a template, inert: nothing of
 * it loads.
 */
function headingsOf(html: string): Heading[] {
  const template = document.createElement("template");
  template.innerHTML = html;
  const headings: Heading[] = [];
  for (const heading of template.content.querySelectorAll<HTMLElement>(
    "h1[id^='nw-'], h2[id^='nw-'], h3[id^='nw-'], h4[id^='nw-'], h5[id^='nw-'], h6[id^='nw-']"
  )) {
    if (heading.closest(".footnotes") !== null) {
      continue;
    }
    const copy = heading.cloneNode(true) as HTMLElement;
    for (const left of copy.querySelectorAll("sup[id^='nw-fnref'], .nw-image > a")) {
      left.remove();
    }
    const text = (copy.textContent ?? "").replace(/\s+/g, " ").trim();
    if (text !== "") {
      headings.push({ id: heading.id, level: Number(heading.tagName.slice(1)), text });
    }
  }
  return headings;
}
