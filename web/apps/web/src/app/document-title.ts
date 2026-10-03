import { useEffect } from "react";

const app = "Nerve Wiki";

/**
 * useDocumentTitle names the tab after the page shown (WCAG 2.4.2): what
 * its heading says, then the places it is in, from the nearest out, then
 * the app, as "Q4 · Plans · Nerve Wiki". Parts not known yet are left
 * out. Once the page goes, the tab is the app's name alone, until the next
 * page names it.
 */
export function useDocumentTitle(...parts: (string | undefined)[]): void {
  const title = [...parts.filter((part) => part !== undefined && part !== ""), app].join(" · ");
  useEffect(() => {
    document.title = title;
    return () => {
      document.title = app;
    };
  }, [title]);
}
