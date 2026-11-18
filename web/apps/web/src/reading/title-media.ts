import type { Enhancement } from "./enhancement";

/**
 * titleMedia keeps a folded callout as it was when a control of an audio
 * or a video in its title is used (M12/P2/S5, 15.2). The server writes a
 * folded callout as <details>, its title the <summary>, and an embed of a
 * medium there as its element with its controls. Chromium opens or closes
 * the callout for a click on the control's blank or on its time, its
 * media controls keeping the click to themselves: on the document there is
 * only the medium's DOMActivate, then the toggle; Firefox and WebKit
 * toggle nothing.
 *
 * As a medium in a callout's own title is activated, the callout's open
 * is noted; in the next animation frame's callbacks, before the frame is
 * painted, it is put back if it changed and the callout is still in the
 * page. Chromium then fires one toggle, of the state put back, and paints
 * no frame open. Nothing is kept from one event to another: the app's own
 * opening later (an anchor's unfold, a focus given back) is not undone.
 * The reader's click on the title's text, or Enter and Space, activate
 * the summary, not a medium, and toggle as ever. The control's own work is
 * not prevented.
 */
export const titleMedia: Enhancement = (container) => {
  container.addEventListener("DOMActivate", keepTitled, true);
  return () => container.removeEventListener("DOMActivate", keepTitled, true);
};

/** keepTitled puts back, before the next frame is painted, the open of the callout whose title's medium event activated. */
function keepTitled(event: Event): void {
  const details = titleDetails(event.target);
  if (details === undefined) {
    return;
  }
  const open = details.open;
  requestAnimationFrame(() => {
    if (details.isConnected && details.open !== open) {
      details.open = open;
    }
  });
}

/** titleDetails is the <details> whose own <summary> holds target, an audio or a video; undefined for any other. */
function titleDetails(target: EventTarget | null): HTMLDetailsElement | undefined {
  if (!(target instanceof HTMLAudioElement || target instanceof HTMLVideoElement)) {
    return undefined;
  }
  const summary = target.closest("summary");
  const details = summary?.parentElement;
  if (!(details instanceof HTMLDetailsElement) || details.querySelector(":scope > summary") !== summary) {
    return undefined;
  }
  return details;
}
