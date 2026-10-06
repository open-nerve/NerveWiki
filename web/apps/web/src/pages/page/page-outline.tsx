import { memo, useMemo } from "react";
import { Link } from "react-router";

import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { usePageView } from "./page-view";
import { PanelSection } from "./panel-section";

/**
 * How many headings the outline lists at most: a page may have a million, a
 * list of links that long the tab's memory.
 */
const listedUpTo = 1000;

/** A heading of the page: its id, its level and its text. */
type Heading = { id: string; level: number; text: string };

/**
 * PageOutline is the page's headings as its reading view has them (M6/P7
 * design 8): from the HTML the view read, read the same way under its key
 * (SWR reads it once for both). Each is indented by its level, from the
 * page's highest, and leads to its heading through the router, as a link
 * of the page to its anchor does: the view has the heading show and take
 * the focus. A page without headings has no outline; one of more than
 * listedUpTo lists the first, and says how many more it has.
 */
export const PageOutline = memo(function PageOutline({ notebook, page }: { notebook: Notebook; page: string }) {
  const t = useT();
  const { data } = usePageView(notebook, page);
  const html = data?.html;
  const headings = useMemo(() => (html === undefined ? [] : headingsOf(html)), [html]);
  if (headings.length === 0) {
    return null;
  }
  const listed = headings.slice(0, listedUpTo);
  // Not by spreading them into Math.min: a call takes so many arguments only.
  const top = listed.reduce((highest, { level }) => Math.min(highest, level), 6);
  return (
    <PanelSection title={t("page.outline")}>
      <nav aria-label={t("page.outline")}>
        <ul className="space-y-1 text-sm">
          {listed.map(({ id, level, text }) => (
            <li key={id} style={{ paddingLeft: `${(level - top) * 0.75}rem` }}>
              <Link to={{ hash: encodeURIComponent(id) }} className="block break-words hover:underline">
                {text}
              </Link>
            </li>
          ))}
        </ul>
        {headings.length > listed.length && (
          <p className="mt-1 text-sm text-muted-foreground">
            {t("page.moreHeadings", { count: String(headings.length - listed.length) })}
          </p>
        )}
      </nav>
    </PanelSection>
  );
});

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
