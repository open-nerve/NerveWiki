/** The keys that are no keyboard's alone: held with the pointer, to copy, to click with Shift, for a screenshot. */
const modifiers = new Set(["Shift", "Meta", "Control", "Alt", "CapsLock"]);

/**
 * trackInputModality keeps on root, as data-modality, how the reader last gave input, "keyboard" or "pointer", and
 * answers what stops it; with no input yet, there is none. A menu's item shows its ring for the keyboard alone (the
 * in-data-[modality=keyboard]: variant): Radix gives the item the pointer moves over the focus, which :focus-visible
 * matches too (M13/P1/S2, decision 760). As WICG's focus-visible does, a key with Meta, Ctrl or Alt, or a modifier
 * alone, is no keyboard's; Shift and Tab is. The document hears the keys and the pointer first, in their capture, so
 * that a control keeping a key to itself keeps it from none of this.
 */
export function trackInputModality(root: HTMLElement): () => void {
  const document = root.ownerDocument;
  const set = (modality: "keyboard" | "pointer") => {
    if (root.dataset.modality !== modality) {
      root.dataset.modality = modality;
    }
  };
  const onKey = (event: KeyboardEvent) => {
    if (!(event.metaKey || event.ctrlKey || event.altKey || modifiers.has(event.key))) {
      set("keyboard");
    }
  };
  const onPointer = () => set("pointer");
  const options = { capture: true, passive: true } as const;
  document.addEventListener("keydown", onKey, options);
  document.addEventListener("pointerdown", onPointer, options);
  document.addEventListener("pointermove", onPointer, options);
  return () => {
    document.removeEventListener("keydown", onKey, options);
    document.removeEventListener("pointerdown", onPointer, options);
    document.removeEventListener("pointermove", onPointer, options);
  };
}
