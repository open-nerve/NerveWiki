import type { Enhancement } from "./enhancement";

/**
 * taskToggle lets the notebook's writers tick and clear the reading view's
 * task items (M5/P6 design 3.5). The server renders every checkbox
 * disabled, with its byte position in data-task; where the context can
 * toggle them, a writer's, they are enabled, each named by its item's
 * visible text (WCAG 2.5.3, 4.1.2), and a click (the space key's among
 * them) asks the server: the checkbox shows the server's state alone,
 * which the view read again brings. The context lets one toggle of the
 * page out at a time: a click meanwhile does nothing. A toggle refused is
 * reported. A reader's checkboxes stay disabled.
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
  const onClick = (event: MouseEvent) => {
    const box = event.target;
    if (!(box instanceof HTMLInputElement) || !boxes.includes(box)) {
      return;
    }
    event.preventDefault();
    // The checked attribute is the server's state; the click asks for the other.
    toggleTask(Number(box.dataset.task), !box.hasAttribute("checked")).catch((error: unknown) => report(error));
  };
  for (const box of boxes) {
    box.disabled = false;
    const text = taskText(box);
    if (text !== "") {
      box.setAttribute("aria-label", text);
    }
  }
  container.addEventListener("click", onClick);
  return () => {
    container.removeEventListener("click", onClick);
    for (const box of boxes) {
      box.disabled = true;
      box.removeAttribute("aria-label");
    }
  };
};

/**
 * taskText is the text of the item box ticks: what follows the box in its
 * paragraph, heading or list item up to the item's next block (a sublist,
 * a code block, a quote), its spaces collapsed; a formula's its TeX,
 * typeset or not, which math keeps (data-tex): the same on every read.
 */
export function taskText(box: Element): string {
  let text = "";
  for (let node = box.nextSibling; node !== null && !(node instanceof Element && blocks.has(node.tagName));) {
    text += textOf(node);
    node = node.nextSibling;
  }
  return text.replace(/\s+/g, " ").trim();
}

/** textOf is node's text, a typeset formula's its TeX. */
function textOf(node: Node): string {
  if (!(node instanceof Element)) {
    return node.textContent ?? "";
  }
  if (node instanceof HTMLElement && node.classList.contains("nw-math") && node.dataset.tex !== undefined) {
    return node.dataset.tex;
  }
  return Array.from(node.childNodes, textOf).join("");
}

/** blocks are the elements a tight list item's next block starts with, as the server renders them. */
const blocks = new Set([
  "BLOCKQUOTE",
  "DETAILS",
  "DIV",
  "DL",
  "FIGURE",
  "H1",
  "H2",
  "H3",
  "H4",
  "H5",
  "H6",
  "HR",
  "OL",
  "P",
  "PRE",
  "TABLE",
  "UL",
]);
