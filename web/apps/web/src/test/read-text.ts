/**
 * readText is element's text as a screen reader has it, without what is hidden from it (aria-hidden): a trail's step
 * without its separator, which ends its link (M13 closeout, decision 968).
 */
export function readText(element: Element): string {
  const read = element.cloneNode(true) as Element;
  for (const hidden of read.querySelectorAll("[aria-hidden]")) {
    hidden.remove();
  }
  return read.textContent ?? "";
}
