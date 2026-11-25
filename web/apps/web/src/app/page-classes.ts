// The classes of the pages that are no settings' and no page's (M13/P5, decision 925): the homes' and the lists'
// columns, their title's row, a group's title; a list's notes, filters and rows; a document's page's head and facts.
// Classes alone, as P3's are: no component.

/** The workspace's home: its cards three abreast from xl (decision 916). */
export const homeColumnClass = "mx-auto w-full max-w-232 space-y-7 md:pt-5 md:pb-4";

/** A list's page and a notebook's home: 720px, centred, as a page's column (decision 916). */
export const listColumnClass = "mx-auto w-full max-w-180 space-y-4 md:pt-5 md:pb-4";

/**
 * A page's title and, on its right, what the page does: New page, Recheck (decision 925). Its title takes
 * cn(pageTitleClass, titleTextClass), its button ms-auto: the button on the right, below the title too.
 */
export const titleRowClass = "flex flex-wrap items-center justify-between gap-x-4 gap-y-3";

/**
 * The title in that row: the width its button leaves, at least 14rem from sm before the button goes below it;
 * under sm as wide as its words, so that a short title keeps its button beside it (review B's P5B-R1-M1).
 */
export const titleTextClass = "min-w-0 flex-1 basis-56 max-sm:basis-auto";

/** A group's title above its cards or rows: 13px, grey (the mockup's settings .gt). */
export const groupTitleClass = "mb-2.5 px-0.5 text-compact font-semibold text-muted-foreground";

/** A line or two under a list page's title: what it holds, how long things stay. */
export const noteClass = "text-compact text-muted-foreground";

/**
 * A list's row of lines: a hit, a changeset, a directory's page (decision 925; P3's stackedRowClass is a line and a
 * block). It is a group, its secondary actions shown as it is hovered or focused (revealClass).
 */
export const stackedLinesClass = "group/row space-y-1 px-4.5 py-3.5";

/**
 * A row of text on the left and what it does on the right: an item of the trash (decision 925). Its text takes P3's
 * listTextClass, the width its button leaves; its button ms-auto, on the right below the text too.
 */
export const splitRowClass = "group/row flex flex-wrap items-start justify-between gap-x-4 gap-y-2 px-4.5 py-3.5";

/**
 * A row's first line, 14px, 500, as a settings row's (review B's P5B-I2); its link, underlined as it is hovered alone
 * (00-M13 4.7: a list's links are not).
 */
export const lineTitleClass = "flex items-center gap-2 text-sm font-medium";
export const lineLinkClass = "min-w-0 break-words underline-offset-4 hover:underline";

/** A row's lines of facts. */
export const factsClass = "text-compact break-words text-muted-foreground";

/** A list page's bar of filters (decision 926). */
export const filterBarClass = "flex flex-wrap items-end gap-x-3 gap-y-3";

/** A filter's label above its control, as FormField spaces them (01-P1 section 9; decision 926). */
export const filterFieldClass = "flex min-w-0 max-w-full flex-col gap-1.5";

/**
 * A filter shown as a chip: the history's page, with its Remove; its corners half its least height, so that it is no
 * pill when it wraps.
 */
export const filterChipClass =
  "flex min-h-9 max-w-full min-w-0 items-center gap-1 rounded-[18px] border border-border bg-surface-2 ps-3 pe-1 text-compact";

/**
 * What a row offers besides its own link, seen as the row is hovered or holds the keyboard's focus, or as its menu is
 * open; always where nothing hovers or a finger may point (as the tree's ⋯, decision 953), on a narrow screen, and in
 * forced colours (decision 927). Transparent, not hidden: Playwright clicks what it sees, and a transparent control is
 * seen. Its transition names opacity and the button's colours both, as the tree's cog does: cn would let
 * transition-opacity take transition-colors' place.
 */
export const revealClass =
  "opacity-0 transition-[opacity,color,background-color] duration-150 group-hover/row:opacity-100 group-focus-within/row:opacity-100 focus-visible:opacity-100 aria-expanded:opacity-100 [@media(hover:none)]:opacity-100 any-pointer-coarse:opacity-100 max-md:opacity-100 forced-colors:opacity-100";

/**
 * The head's right on a document's page: what the page does (Restore this version, Undo change); the page's own is
 * P4's.
 */
export const headActionsClass = "ms-auto flex min-w-0 flex-wrap items-center justify-end gap-1.5";

/**
 * The facts under a document's title, one look on the three pages (review B's P5B-M9): grey, its links a shade
 * darker.
 */
export const docFactsClass = "text-[12.5px] text-faint [&_a]:text-muted-foreground";
