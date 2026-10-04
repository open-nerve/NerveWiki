import { useEffect } from "react";

import { useStore } from "../stores/context";

/**
 * SignOutElsewhere closes this tab's edits as another tab of the login
 * signs out, each once what it has unsaved is saved (M4–M5 Codex review
 * R2): the logout that follows ends the session they save with. It sits
 * with the providers, mounted anew with each generation.
 */
export function SignOutElsewhere() {
  const store = useStore();
  useEffect(() => store.answerSignOuts(), [store]);
  return null;
}
