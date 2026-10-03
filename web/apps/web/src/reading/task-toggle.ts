import type { Enhancement } from "./enhancement";

/**
 * taskToggle lets the notebook's writers tick and clear the reading view's
 * task items (M5/P6 design 3.5). The server renders every checkbox
 * disabled, with its byte position in data-task; where the context can
 * toggle them, a writer's, they are enabled, and a click (the space key's
 * among them) asks the server: the checkbox shows the server's state
 * alone, which the view read again brings. One toggle is out at a time: a
 * click meanwhile does nothing. A toggle refused is reported above the
 * view. A reader's checkboxes stay disabled.
 */
export const taskToggle: Enhancement = (container, context) => {
  const { toggleTask, report } = context;
  if (toggleTask === undefined) {
    return undefined;
  }
  const boxes = [...container.querySelectorAll<HTMLInputElement>("input[data-task]")];
  if (boxes.length === 0) {
    return undefined;
  }
  let out = false;
  const onClick = (event: MouseEvent) => {
    const box = event.target;
    if (!(box instanceof HTMLInputElement) || !boxes.includes(box)) {
      return;
    }
    event.preventDefault();
    if (out) {
      return;
    }
    out = true;
    // The checked attribute is the server's state; the click asks for the other.
    toggleTask(Number(box.dataset.task), !box.hasAttribute("checked"))
      .catch((error: unknown) => report(error))
      .finally(() => {
        out = false;
      });
  };
  for (const box of boxes) {
    box.disabled = false;
  }
  container.addEventListener("click", onClick);
  return () => {
    container.removeEventListener("click", onClick);
    for (const box of boxes) {
      box.disabled = true;
    }
  };
};
