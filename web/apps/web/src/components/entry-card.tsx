import type { ReactNode } from "react";

import { appName } from "../app/document-title";

/** The title of a page in an entry card, and of a status page: 22px (decisions 920, 921). */
export const entryTitleClass = "text-[22px] leading-snug font-[650] tracking-[-0.01em] break-words outline-none";

/**
 * EntryCard is the card of an entry page (M13/P5/S1, decision 920): sign-in, sign-up, an invitation, the onboarding
 * and a new workspace, centred, 400px wide at most. At its top the product's mark, the favicon's (inline, no request;
 * its square the ink, its letter the ink's foreground, light or dark as the theme; read by no one, hidden in forced
 * colours) and its name, in text: a page has one heading, its h1, below it with the page's own content.
 */
export function EntryCard({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-col items-center pt-[4svh] md:pt-[10svh]">
      <section className="w-full max-w-100 rounded-card border border-border bg-surface px-6 py-7 sm:px-8 sm:py-8">
        <div className="mb-6 flex items-center gap-2.5 text-sm font-semibold">
          <svg aria-hidden viewBox="0 0 32 32" className="size-7 shrink-0 forced-colors:hidden">
            <rect width="32" height="32" rx="7" className="fill-primary" />
            <path d="M9 23V9h2.6l7.8 9.6V9H22v14h-2.6l-7.8-9.6V23z" className="fill-primary-foreground" />
          </svg>
          {appName}
        </div>
        <div className="space-y-6">{children}</div>
      </section>
    </div>
  );
}
