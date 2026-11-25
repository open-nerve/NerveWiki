// The classes of the pages that are no settings' and no page's (M13/P5, decision 925): the homes' columns and title's
// row, a group's title. Classes alone, as P3's are: no component.

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
