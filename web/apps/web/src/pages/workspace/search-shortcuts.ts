import { useEffect, useState, type RefObject } from "react";

import type { DrawerActions } from "../../app/shell-sidebar";
import { composing, dialogOpen, isMod, isModShift } from "../../app/shortcuts";
import { onMac } from "../../lib/platform";

/**
 * useSearchShortcuts has Mod+K and Mod+Shift+F (Cmd on macOS, Ctrl elsewhere) give the workspace's search box, field,
 * the focus, its text selected, instead of the browser's own use of the keys (M8/P5 design 4.2; M13/P2/S3, decision
 * 783; M5-polish 26.3): the shell's, WorkspaceLayout's, which has the box in its left column. Not while a key is held,
 * nor in a composition, whose half-typed text the browser would write into the page as the focus left it (v0.1 design
 * 13.2, item 20); over a dialog, or a menu, it only keeps the browser's use of them: a drawer opened under a menu
 * would leave the focus behind it, at a phone's width (decision 806). Mod+K is the editor's too, its link's (its Mod-K,
 * which has the key first): one taken already, or pressed in an editor, it leaves alone. Without a box, the workspace
 * not loaded or not found, neither is its: the browser's. The drawer of a phone's width is no dialog to them
 * (besideDrawer): open, they focus the box in it; closed, they open it at the box (actions, useDrawer's, the same for
 * the shell's life, which read the drawer as they are called: the listener is added once).
 */
export function useSearchShortcuts({
  field,
  actions,
}: {
  field: RefObject<HTMLInputElement | null>;
  actions: DrawerActions;
}): void {
  const [mac] = useState(onMac);
  useEffect(() => {
    const onShortcut = (event: KeyboardEvent) => {
      const k = isMod(event, "k", mac);
      if ((!k && !isModShift(event, "f", mac)) || composing(event) || field.current === null) {
        return;
      }
      if (k && (event.defaultPrevented || document.activeElement?.closest(".cm-editor"))) {
        return;
      }
      event.preventDefault();
      if (event.repeat || dialogOpen({ besideDrawer: true }) || document.querySelector('[role="menu"]') !== null) {
        return;
      }
      if (actions.closed()) {
        actions.open("search");
      } else {
        field.current.focus();
        field.current.select();
      }
    };
    window.addEventListener("keydown", onShortcut);
    return () => window.removeEventListener("keydown", onShortcut);
  }, [mac, field, actions]);
}
