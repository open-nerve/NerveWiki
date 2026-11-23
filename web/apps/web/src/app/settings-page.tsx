import { observer } from "mobx-react-lite";
import { createContext, use, useId, useState, type ReactNode, type RefObject } from "react";

import { useArrivalFocus } from "./arrival";

/** The shown settings page's h1, which a row removed with the focus in it gives the focus to (decision 815). */
const SettingsHeading = createContext<RefObject<HTMLHeadingElement | null> | undefined>(undefined);

/** useSettingsHeading is the h1 of the settings page shown: only below SettingsPage. */
export function useSettingsHeading(): RefObject<HTMLHeadingElement | null> {
  const heading = use(SettingsHeading);
  if (heading === undefined) {
    throw new Error("useSettingsHeading is used outside SettingsPage");
  }
  return heading;
}

/**
 * SettingsPage is a page of the settings' mode (00-M13 4.4.2; M13/P3/S1, decisions 815, 820, 844): its name as its h1,
 * which takes the arrival focus as it mounts (the layouts key it by the page: arrived at twice in one layout, from the
 * drawer, each page's h1 mounts anew and takes it), above its groups. Centred, its content at most 696px wide (the
 * mockup's .page, 760px with 32px each side), the main's 24px of padding made the mockup's 44 / 32 / 40 from md: up
 * (P4 decides the main's own).
 */
export function SettingsPage({ title, children }: { title: string; children: ReactNode }) {
  const heading = useArrivalFocus<HTMLHeadingElement>();
  return (
    <div className="mx-auto w-full max-w-178 md:px-2 md:pt-5 md:pb-4">
      <h1
        ref={heading}
        tabIndex={-1}
        className="mb-7 text-[26px] leading-normal font-[650] tracking-[-0.01em] break-words outline-none"
      >
        {title}
      </h1>
      <SettingsHeading value={heading}>
        <div className="space-y-6.5">{children}</div>
      </SettingsHeading>
    </div>
  );
}

/**
 * SettingsSection is a group of a settings page (the mockup's .sec): its title, if any, a heading, 13px and grey, 10px
 * above its card, the groups 26px apart (SettingsPage); heading given, the title is focusable, where an action gives
 * it the focus.
 */
export function SettingsSection({
  title,
  heading,
  children,
}: {
  title?: string;
  heading?: RefObject<HTMLHeadingElement | null>;
  children: ReactNode;
}) {
  return (
    <section className="space-y-2.5">
      {title !== undefined && (
        <h2
          ref={heading}
          tabIndex={heading === undefined ? undefined : -1}
          className="px-0.5 text-compact font-semibold text-muted-foreground outline-none"
        >
          {title}
        </h2>
      )}
      {children}
    </section>
  );
}

/** SettingsCard is a group's card: 14px corners, a line between its rows, no shadow; list, a list of them. */
export function SettingsCard({ list = false, children }: { list?: boolean; children: ReactNode }) {
  const className = "divide-y divide-border rounded-card border border-border bg-surface";
  return list ? <ul className={className}>{children}</ul> : <div className={className}>{children}</div>;
}

/** A card's row (the mockup's .set): its text on the left, its control on the right, under the text below md. */
export const rowClass = "flex min-h-16 flex-wrap items-center gap-x-6 gap-y-2.5 px-4.5 py-3.5";
/** The row's text: its title, its description. */
export const rowTextClass = "min-w-0 flex-1 basis-56";
export const rowTitleClass = "text-sm font-medium";
export const rowDescriptionClass = "mt-0.5 max-w-105 text-[12.5px] leading-normal text-faint";
/** The row's controls, which do not shrink: on the right from md: up, the row's width below it. */
export const rowControlClass = "flex min-w-0 flex-wrap items-center gap-2 max-md:w-full md:ml-auto md:shrink-0";
/**
 * A value the row shows alone, in the controls' stead: at most 60% of the row from md: up, on the right, shrinking,
 * broken where it is long (review B's M4). md:text-right: text-end would read as a colour (design-tokens.test.ts).
 * SettingsRow's alone (its value), not exported.
 */
const rowValueClass =
  "min-w-0 text-sm break-words text-muted-foreground max-md:w-full md:ml-auto md:max-w-[60%] md:text-right";
/**
 * A field's problem, a line of its own in the controls: as wide as they are, counted as none, the gap before it taken
 * back from md: up, so that it widens them not (review B's R1-M1).
 */
export const rowProblemClass = "w-0 min-w-full basis-full text-compact text-destructive md:-me-2";

/** The ids of a row's title and description, which name and describe a group of its controls. */
type RowIds = { title: string; description: string | undefined };

/**
 * SettingsRow is a card's row that shows or acts (00-M13 4.4.2): its title, a paragraph, and its description; then a
 * value it shows (value, rowValueClass), or its controls (children, given the ids of the title and the description, to
 * name a radio group by). A row without a title says what it is in its description, in the text's size; a field's row
 * labels its field itself (NameRow). An observer: children is called in its render, so that what the controls read
 * of a store, it follows (decision 849; a8 found the appearance's radios not following it otherwise).
 */
export const SettingsRow = observer(function SettingsRow({
  title,
  description,
  value,
  children,
}: {
  title?: ReactNode;
  description?: ReactNode;
  value?: ReactNode;
  children?: (ids: RowIds) => ReactNode;
}) {
  const id = useId();
  const ids: RowIds = {
    title: `${id}-title`,
    description: description === undefined ? undefined : `${id}-description`,
  };
  return (
    <div className={rowClass}>
      <div className={rowTextClass}>
        {title !== undefined && (
          <p id={ids.title} className={rowTitleClass}>
            {title}
          </p>
        )}
        {description !== undefined && (
          <p
            id={ids.description}
            className={title === undefined ? "text-sm text-muted-foreground" : rowDescriptionClass}
          >
            {description}
          </p>
        )}
      </div>
      {value !== undefined && <p className={rowValueClass}>{value}</p>}
      {children !== undefined && <div className={rowControlClass}>{children(ids)}</div>}
    </div>
  );
});

/**
 * SavedOutput is a row's "Saved." (v0.1 design 13.2 item 12; decisions 823, 841, 848): an output always mounted, its
 * text the label or none, first of its row's controls (below md, last): what follows it does not move as it shows,
 * its 68px kept from md: up, empty too. Its check is styles.css's (nw-saved), drawn before the text, never in it; it
 * fades 1.5s after it shows. Each time it is written its round turns, which names the fade's other keyframes: the
 * fade starts again however soon the text comes back after the output was emptied.
 */
export function SavedOutput({ saved, label }: { saved: boolean; label: string }) {
  const [round, setRound] = useState(0);
  const [was, setWas] = useState(saved);
  if (was !== saved) {
    setWas(saved);
    if (saved) {
      setRound((n) => 1 - n);
    }
  }
  return (
    <output
      data-round={round}
      className="nw-saved inline-flex items-center gap-1 text-xs whitespace-nowrap text-faint max-md:order-last max-md:empty:-me-2 md:min-w-17 md:justify-end"
    >
      {saved ? label : ""}
    </output>
  );
}
