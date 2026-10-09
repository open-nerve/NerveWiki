/** unfold opens the folded callouts (closed details) element is in, which could show nothing of it otherwise. */
export function unfold(element: HTMLElement) {
  for (let parent = element.parentElement; parent !== null; parent = parent.parentElement) {
    if (parent instanceof HTMLDetailsElement && !parent.open) {
      parent.open = true;
    }
  }
}
