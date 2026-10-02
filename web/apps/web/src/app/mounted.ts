import { useCallback, useEffect, useRef } from "react";

/**
 * useMounted tells whether the component is still mounted. A write whose
 * answer changes the route (a creation, an acceptance) goes nowhere once
 * the page it was sent from is gone: the user left it, for another page or
 * another workspace, and is not to be taken back. The store keeps the
 * write's change: the lists show what was created (v0.1 design 13.2, item
 * 16; M3 Codex review R3). It is the page's, or the part of the shell's,
 * whose going means the user left: not a part the write itself may end.
 */
export function useMounted(): () => boolean {
  const mounted = useRef(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  return useCallback(() => mounted.current, []);
}
