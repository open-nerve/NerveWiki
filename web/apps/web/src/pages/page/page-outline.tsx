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
 * highest of those listed, and leads to its heading through the router, as
 * a link of the page to its anchor does: the view has the heading show and
 * take the focus. A page without headings has no outline; one of more than
 * listedUpTo lists the first, and says how many more it has. It renders
 * again as its view does, not as the column does: the notebooks read again
 * give it the same notebook anew, by its id.
 */
export const PageOutline = memo(function PageOutline({ notebook, page }: { notebook: Notebook; page: string }) {
  const t = useT();
  const { data } = usePageView(notebook, page);
  const html = data?.html;
  const { listed, more } = useMemo(() => (html === undefined ? { listed: [], more: 0 } : headingsOf(html)), [html]);
  if (listed.length === 0) {
    return null;
  }
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
        {more > 0 && (
          <p className="mt-1 text-sm text-muted-foreground">{t("page.moreHeadings", { count: String(more) })}</p>
        )}
      </nav>
    </PanelSection>
  );
}, sameView);

/** sameView tells whether the outline's props are those of the same page's view: a notebook's by its id. */
function sameView(before: { notebook: Notebook; page: string }, after: { notebook: Notebook; page: string }): boolean {
  return before.notebook.id === after.notebook.id && before.page === after.page;
}

/** attachmentElements are an attachment's elements, whose text is an attribute. */
const attachmentElements = "img.nw-asset, audio.nw-asset, video.nw-asset";

/**
 * headingsOf is the headings of a page's HTML that an anchor leads to: those
 * with an id the server gave (nw-), in their order, but those of the
 * footnotes, with their text: a formula's is its TeX, as the server writes
 * it, an attachment's image, audio or video its text (alt, aria-label); a
 * footnote's number and an
 * image's address are not. One without
 * text is left out, as it would be a link to nothing one could read. The
 * first listedUpTo are listed; the rest only counted, as they are (one
 * whose text is a footnote's number or an image's address alone counts),
 * not copied: a page may have a million. The HTML, the server's sanitized,
 * is parsed in a template, inert: nothing of it loads.
 */
function headingsOf(html: string): { listed: Heading[]; more: number } {
  const template = document.createElement("template");
  template.innerHTML = html;
  const listed: Heading[] = [];
  let more = 0;
  for (const heading of template.content.querySelectorAll<HTMLElement>(
    "h1[id^='nw-'], h2[id^='nw-'], h3[id^='nw-'], h4[id^='nw-'], h5[id^='nw-'], h6[id^='nw-']"
  )) {
    if (heading.closest(".footnotes") !== null) {
      continue;
    }
    if (listed.length === listedUpTo) {
      if ((heading.textContent ?? "").trim() !== "" || heading.querySelector(attachmentElements) !== null) {
        more++;
      }
      continue;
    }
    const copy = heading.cloneNode(true) as HTMLElement;
    for (const left of copy.querySelectorAll("sup[id^='nw-fnref'], .nw-image > a")) {
      left.remove();
    }
    for (const element of copy.querySelectorAll(attachmentElements)) {
      element.replaceWith((element.getAttribute(element.tagName === "IMG" ? "alt" : "aria-label") ?? "").trim());
    }
    const text = (copy.textContent ?? "").replace(/\s+/g, " ").trim();
    if (text !== "") {
      listed.push({ id: heading.id, level: Number(heading.tagName.slice(1)), text });
    }
  }
  return { listed, more };
}
