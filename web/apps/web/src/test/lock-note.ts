// The edit lock's note, as the page's tests find it (M5/P3 design 3.10).

/**
 * lockNote matches the lock's note saying text, in full: its output's text, the holder's name in a bdi too, which
 * Testing Library's text of an element leaves out (its own text nodes only; M13/P4/S1, decision 868).
 */
export function lockNote(text: string): (content: string, element: Element | null) => boolean {
  return (_, element) => element?.tagName === "OUTPUT" && element.textContent === text;
}
