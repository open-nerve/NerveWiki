/** onMac tells whether the browser runs on macOS, where Mod is Cmd; elsewhere it is Ctrl. */
export function onMac(platform = navigator.platform): boolean {
  return /^mac/i.test(platform);
}
