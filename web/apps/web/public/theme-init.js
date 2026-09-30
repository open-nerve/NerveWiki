// Sets the dark class on <html> from the stored theme preference before the
// first paint (index.html loads this file synchronously in <head>), so that a
// dark page does not flash light first. The key and the rule are
// PreferencesStore's (src/stores/preferences.store.ts); a test holds them
// together. It is a file rather than an inline script: the pages' CSP allows
// scripts from nervewiki only.
{
  let theme = null;
  try {
    theme = window.localStorage.getItem("nwiki.theme");
  } catch {
    // storage blocked: follow the system
  }
  if (theme !== "light" && theme !== "dark") {
    theme = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  document.documentElement.classList.toggle("dark", theme === "dark");
}
