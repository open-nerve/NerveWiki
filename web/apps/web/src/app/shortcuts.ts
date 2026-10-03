/** The keys of a key press that a shortcut reads. */
type Keys = Pick<KeyboardEvent, "key" | "code" | "metaKey" | "ctrlKey" | "altKey" | "shiftKey">;

/** onMac tells whether the browser runs on macOS, where Mod is Cmd; elsewhere it is Ctrl. */
export function onMac(platform = navigator.platform): boolean {
  return /^mac/i.test(platform);
}

/**
 * isMod tells whether keys are Mod and the letter key alone (M4/P5 design
 * 3.10): Cmd on macOS, Ctrl elsewhere, with no other modifier. On a layout
 * whose letters are not Latin (Cyrillic, Greek) the letter is the Latin one
 * at the key's place, as the browsers' own shortcuts take it.
 */
export function isMod(keys: Keys, key: string, mac: boolean): boolean {
  const mod = mac ? keys.metaKey && !keys.ctrlKey : keys.ctrlKey && !keys.metaKey;
  const letter = /^[a-z]$/i.test(keys.key)
    ? keys.key.toLowerCase()
    : /^Key[A-Z]$/.exec(keys.code)?.[0].slice(3).toLowerCase();
  return mod && !keys.altKey && !keys.shiftKey && letter === key;
}
