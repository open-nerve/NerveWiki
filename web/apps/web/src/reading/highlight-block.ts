import hljs from "highlight.js/lib/common";

import type { HighlightAnswer, HighlightRequest } from "./highlight";

/**
 * highlightBlock is the worker's answer for a block: highlight.js's HTML
 * for its language among the common ones, or null for another language.
 */
export function highlightBlock({ id, language, text }: HighlightRequest): HighlightAnswer {
  if (hljs.getLanguage(language) === undefined) {
    return { id, html: null };
  }
  return { id, html: hljs.highlight(text, { language, ignoreIllegals: true }).value };
}
