import { useSyncExternalStore } from "react";

import type { SessionState } from "../stores/auth.store";
import { useStore } from "../stores/context";

/**
 * useSession is the tab's session state; the component renders again on every change. Its own module, not the guards':
 * the top right reads it too, and the guards' module leads to it (guards, session-unavailable, corner), a cycle
 * (M13/P2/S1, decision 793).
 */
export function useSession(): SessionState {
  const { auth } = useStore();
  return useSyncExternalStore(auth.subscribe, () => auth.state);
}
