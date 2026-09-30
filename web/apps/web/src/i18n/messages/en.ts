// The source of the app's text: its keys are the keys of every language.
export const en = {
  "theme.label": "Theme",
  "theme.system": "System",
  "theme.light": "Light",
  "theme.dark": "Dark",
  "language.label": "Language",
  "home.version": "Version {version} ({commit})",
  "home.apiVersion": "API {apiVersion}",
  "home.loading": "Loading…",
  "home.loadFailed": "The instance information could not be loaded.",
  "error.title": "Something went wrong",
  "error.body": "This page could not be shown.",
  "error.code": "Error code: {code}",
  "error.reload": "Reload",
  "notFound.title": "Page not found",
  "notFound.body": "There is no page at this address.",
  "notFound.home": "Go to the home page",
} as const;

export type MessageKey = keyof typeof en;

/** Messages is a language's text: a string for every key, and no other key. */
export type Messages = Record<MessageKey, string>;
