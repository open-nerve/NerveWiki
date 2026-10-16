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
  return modOf(keys, mac) && !keys.altKey && !keys.shiftKey && letterOf(keys) === key;
}

/**
 * isModShift tells whether keys are Mod, Shift and the letter key (M8/P5
 * design 4.2): Cmd on macOS, Ctrl elsewhere, with Shift and no other
 * modifier; the letter as isMod reads it.
 */
export function isModShift(keys: Keys, key: string, mac: boolean): boolean {
  return modOf(keys, mac) && !keys.altKey && keys.shiftKey && letterOf(keys) === key;
}

/** modOf tells whether keys hold Mod: Cmd without Ctrl on macOS, Ctrl without Cmd elsewhere. */
function modOf(keys: Keys, mac: boolean): boolean {
  return mac ? keys.metaKey && !keys.ctrlKey : keys.ctrlKey && !keys.metaKey;
}

/** letterOf is the letter of keys' key, lower case: its own when Latin, else the Latin one at its place. */
function letterOf(keys: Keys): string | undefined {
  return /^[a-z]$/i.test(keys.key) ? keys.key.toLowerCase() : /^Key[A-Z]$/.exec(keys.code)?.[0].slice(3).toLowerCase();
}

/** dialogOpen tells whether a dialog is open: the page's own shortcuts then leave the keys to it. */
export function dialogOpen(): boolean {
  return document.querySelector('[role="dialog"], [role="alertdialog"]') !== null;
}
