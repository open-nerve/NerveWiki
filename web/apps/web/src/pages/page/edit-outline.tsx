import { List } from "lucide-react";
import { makeAutoObservable, observableRef } from "mobx";
import { observer } from "mobx-react-lite";

import type { OutlineHeading, OutlineSink } from "../../editor/edit-outline";
import { useT } from "../../i18n/i18n";
import { listedUpTo } from "./page-outline";
import { PanelSection } from "./panel-section";

/**
 * OutlineBox is the edit's outline as its editor last told it, and the way to a heading's line (M13/P4/S3, decision
 * 876): the page's shell has one, which its edit hands its editor and its right column draws. The editor's types alone
 * are imported: CodeMirror stays in the editor's chunk.
 */
export class OutlineBox implements OutlineSink {
  readonly upTo = listedUpTo;
  /** The headings listed, as told: the editor tells another list, never changes one. */
  listed: readonly OutlineHeading[] = [];
  more = 0;
  /** reveal leads to the index-th heading's line, the editor focused: the editor's, while there is one. */
  reveal: ((index: number) => void) | undefined = undefined;

  constructor() {
    makeAutoObservable(this, { upTo: false, reveal: false, listed: observableRef });
  }

  told(listed: readonly OutlineHeading[], more: number) {
    this.listed = listed;
    this.more = more;
  }

  clear() {
    this.listed = [];
    this.more = 0;
  }
}

/**
 * EditOutline is the right column's outline while the page is edited (M13/P4/S3, decision 876; 00-M13 4.5.4): the
 * editor's headings, as the reading's outline has the view's, each a button that puts the cursor at its line, the
 * editor focused; not marking the section read. Indented as the reading's, from the highest listed; its items by their
 * index, so that a change of the text moving a heading keeps the button that has the focus (review A's M2). None while
 * the editor tells none.
 */
export const EditOutline = observer(function EditOutline({ box }: { box: OutlineBox }) {
  const t = useT();
  const { listed, more } = box;
  if (listed.length === 0) {
    return null;
  }
  // Not by spreading them into Math.min: a call takes so many arguments only.
  const top = listed.reduce((highest, { level }) => Math.min(highest, level), 6);
  return (
    <PanelSection title={t("page.outline")} icon={List}>
      <nav aria-label={t("page.outline")}>
        <ul className="border-l border-border text-[13px]">
          {listed.map(({ level, text }, index) => (
            <li
              // oxlint-disable-next-line react/no-array-index-key -- the place in the list: a heading moved keeps it
              key={index}
              style={{ paddingLeft: `${((level - top) * 0.75).toString()}rem` }}
              className="-ms-px border-l-2 border-transparent py-1 forced-colors:border-l-0"
            >
              <button
                type="button"
                onClick={() => box.reveal?.(index)}
                className="w-full ps-3 pe-1 text-start leading-snug text-muted-foreground hover:text-foreground"
              >
                {/* Two lines at most on the text's own box: WebKit lays out no button as the clamp's -webkit-box. */}
                <span className="break-words line-clamp-2">{text}</span>
              </button>
            </li>
          ))}
        </ul>
        {more > 0 && (
          <p className="mt-1.5 ps-3 text-xs text-faint">{t("page.moreHeadings", { count: String(more) })}</p>
        )}
      </nav>
    </PanelSection>
  );
});
