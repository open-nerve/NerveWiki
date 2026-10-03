/** The keys of a key press that a shortcut reads. */
type Keys = Pick<KeyboardEvent, "key" | "metaKey" | "ctrlKey" | "altKey" | "shiftKey">;

/** onMac tells whether the browser runs on macOS, where Mod is Cmd; elsewhere it is Ctrl. */
export function onMac(platform = navigator.platform): boolean {
  return /^mac/i.test(platform);
}

/**
 * isMod tells whether keys are Mod and key alone (M4/P5 design 3.10): Cmd
 * on macOS, Ctrl elsewhere, with no other modifier.
 */
export function isMod(keys: Keys, key: string, mac: boolean): boolean {
  const mod = mac ? keys.metaKey && !keys.ctrlKey : keys.ctrlKey && !keys.metaKey;
  return mod && !keys.altKey && !keys.shiftKey && keys.key.toLowerCase() === key;
}
