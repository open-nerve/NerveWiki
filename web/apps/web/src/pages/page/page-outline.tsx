import { List } from "lucide-react";
import { memo, useCallback, useLayoutEffect, useMemo, useRef, type MouseEvent } from "react";
import { Link } from "react-router";

import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import type { Notebook } from "../../services/notebook.service";
import { useCurrentHeading } from "./current-heading";
import { usePageView } from "./page-view";
import { PanelSection } from "./panel-section";
import { watchReader } from "./readers-input";

/**
 * How many headings the outline lists at most: a page may have a million, a
 * list of links that long the tab's memory.
 */
const listedUpTo = 1000;

/** A heading of the page: its id, its level and its text. */
type Heading = { id: string; level: number; text: string };

type OutlineProps = {
  notebook: Notebook;
  page: string;
  /** Whether the section read is measured and marked: the right column open beside the page (decision 886). */
  active: boolean;
};

/**
 * PageOutline is the page's headings as its reading view has them (M6/P7
 * design 8): from the HTML the view read, read the same way under its key
 * (SWR reads it once for both). Each is indented by its level, from the
 * highest of those listed, and leads to its heading through the router, as
 * a link of the page to its anchor does: the view has the heading show and
 * take the focus. A page without headings has no outline; one of more than
 * listedUpTo lists the first, and says how many more it has. It renders
 * again as its view does, or as it becomes active or not, not as the column
 * does: the notebooks read again give it the same notebook anew, by its id.
 *
 * Active, it marks the section read (M13/P4/S1, decisions 861, 886, 893;
 * useCurrentHeading over the headings listed), the amber line on its left
 * and aria-current="location", not moving the focus; a heading clicked
 * (no modifier: one opens a tab, a window or a download, the view not
 * scrolled) at once. The right column scrolling itself, it follows the
 * section read only as far as shows its item, never by scrollIntoView,
 * until the reader moves in it (wheel, touch, a press but on the outline's
 * items, a key with the focus in it): from then on, while its list is up,
 * no more. A long heading shows two lines at most, all of it its name.
 */
export const PageOutline = memo(function PageOutline({ notebook, page, active }: OutlineProps) {
  const t = useT();
  const { data } = usePageView(notebook, page);
  const html = data?.html;
  const { listed, more } = useMemo(() => (html === undefined ? { listed: [], more: 0 } : headingsOf(html)), [html]);
  // Only those listed are measured (review C's M10), the same array until the view's HTML changes.
  const ids = useMemo(() => listed.map(({ id }) => id), [listed]);
  const { current, pin } = useCurrentHeading(ids, active);
  const list = useRef<HTMLUListElement>(null);
  // The reader moved in the right column (decision 886): from then on, while this list is up, it follows no more.
  const reader = useRef<ReturnType<typeof watchReader>>(undefined);
  // Watched from the list's own mounting: the page's view, and the outline with it, may come after the page (review
  // A's R1-I2).
  const watched = useCallback((element: HTMLUListElement | null) => {
    list.current = element;
    if (element === null) {
      return undefined;
    }
    const watching = watchReader({ on: element, within: "[data-page-panel]" });
    reader.current = watching;
    return () => {
      watching.end();
      reader.current = undefined;
      list.current = null;
    };
  }, []);
  // As far as shows the item of the section read, the right column's own scroll alone (v0.1 design 13.2, item 26).
  useLayoutEffect(() => {
    const column = list.current?.closest("aside");
    if (current === undefined || reader.current?.acted() !== false || column === null || column === undefined) {
      return;
    }
    if (column.scrollHeight <= column.clientHeight) {
      return;
    }
    const item = list.current?.querySelector("[data-current]");
    if (item === null || item === undefined) {
      return;
    }
    const shown = column.getBoundingClientRect();
    const at = item.getBoundingClientRect();
    // Whole pixels, up: a scroll offset is kept rounded, and a fraction short leaves the item's edge cut.
    if (at.top < shown.top) {
      column.scrollTop -= Math.ceil(shown.top - at.top);
    } else if (at.bottom > shown.bottom) {
      column.scrollTop += Math.ceil(at.bottom - shown.bottom);
    }
  }, [current]);
  if (listed.length === 0) {
    return null;
  }
  // Not by spreading them into Math.min: a call takes so many arguments only.
  const top = listed.reduce((highest, { level }) => Math.min(highest, level), 6);
  return (
    <PanelSection title={t("page.outline")} icon={List}>
      <nav aria-label={t("page.outline")}>
        <ul ref={watched} className="border-l border-border text-[13px]">
          {listed.map(({ id, level, text }) => (
            <OutlineItem key={id} id={id} indent={level - top} text={text} current={id === current} pin={pin} />
          ))}
        </ul>
        {more > 0 && (
          <p className="mt-1.5 ps-3 text-xs text-faint">{t("page.moreHeadings", { count: String(more) })}</p>
        )}
      </nav>
    </PanelSection>
  );
}, sameView);

/**
 * OutlineItem is a heading of the outline, drawn again only as it becomes or stops being the current one (review A's
 * N6). The line on the outline's left is its list's; the current one's 2px of amber over it, its underline in forced
 * colours, which take the borders' colours away. Indented by its padding, the line kept at the list's left (the
 * mockup's .outline li.sub); its padding on the item, not on the link, which two lines clamp (review B's M1).
 */
const OutlineItem = memo(function OutlineItem({
  id,
  indent,
  text,
  current,
  pin,
}: {
  id: string;
  indent: number;
  text: string;
  current: boolean;
  pin: (id: string, link: Element) => void;
}) {
  return (
    <li
      data-current={current || undefined}
      style={{ paddingLeft: `${indent * 0.75}rem` }}
      className={cn(
        "-ms-px border-l-2 py-1 forced-colors:border-l-0",
        current ? "border-accent" : "border-transparent"
      )}
    >
      <Link
        to={{ hash: encodeURIComponent(id) }}
        aria-current={current ? "location" : undefined}
        onClick={(event) => {
          if (plainPress(event)) {
            pin(id, event.currentTarget);
          }
        }}
        className={cn(
          "ps-3 pe-1 leading-snug break-words line-clamp-2",
          current
            ? "font-medium text-foreground forced-colors:underline"
            : "text-muted-foreground hover:text-foreground"
        )}
      >
        {text}
      </Link>
    </li>
  );
});

/**
 * plainPress is a press of the main button with no modifier, as the router's isModifiedEvent: with one it opens a
 * tab, a window or a download, and the view does not scroll to it (review A's R2-N1).
 */
const plainPress = (event: MouseEvent) =>
  event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;

/** sameView tells whether the outline's props are those of the same page's view, measured or not (review A's R2-I1). */
function sameView(before: OutlineProps, after: OutlineProps): boolean {
  return before.notebook.id === after.notebook.id && before.page === after.page && before.active === after.active;
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
