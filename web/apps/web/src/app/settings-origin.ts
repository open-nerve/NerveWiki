import { createContext, use, useState } from "react";
import { useLocation, useMatches, useNavigationType, useParams, type Location } from "react-router";

/**
 * Origin is where the settings were entered from: the page shown before them, its path, query and fragment, and the
 * workspace it was in (its slug), if any (M13/P3/S1, decisions 812, 834).
 */
export type Origin = { pathname: string; search: string; hash: string; workspace: string | undefined };

/** What a route says of the settings: settings, that its pages are theirs, its left column their directory. */
type SettingsHandle = { settings?: boolean };

/**
 * useSettingsMode tells whether the route shown is a settings page's (handle: { settings: true }, decision 811), as the
 * render reads it: an effect would draw the workspace's column for a paint first.
 */
export function useSettingsMode(): boolean {
  return useMatches().some((match) => (match.handle as SettingsHandle | undefined)?.settings === true);
}

/** The location Layout rendered last, the workspace it was in, whether the settings showed then, the origin kept. */
type Seen = { location: Location; workspace: string | undefined; settings: boolean; origin: Origin | undefined };

/**
 * useOriginRecord is the origin of the settings shown, which Layout gives below it (SettingsOrigin): recorded as the
 * address enters the settings from another page, kept as it goes from one settings page to another (the account's,
 * a workspace's, a notebook's: the directory mounts anew between them). React Router's location is a new object for
 * each navigation, the same through its renders: the last one seen tells a navigation from a render of the same one,
 * and the workspace it was in is that render's slug (Layout's params are the whole match's). Entered by a replace, the
 * settings have none: only a redirect enters them so (the guard's next once signed in, the onboarding's next), the
 * page it replaced no place to go back to, which would send the tab back here at once (decision 833). Taken as the
 * render takes it, as useDrawer takes the window's width: an effect would render the directory once without it. Not
 * in the route's state: the settings' indexes replace themselves with their first page, and Back and Forward would
 * carry it about. Layout's state, for the tab's life: a reload has none. Out of the settings it stays, read by no one
 * (the directory is the settings'), and is taken anew as they are entered again (review C's M1).
 */
export function useOriginRecord(settings: boolean): Origin | undefined {
  const location = useLocation();
  const type = useNavigationType();
  const { slug } = useParams();
  const [seen, setSeen] = useState<Seen>({ location, workspace: slug, settings, origin: undefined });
  if (seen.location !== location) {
    let origin = seen.origin;
    if (settings && !seen.settings) {
      const { pathname, search, hash } = seen.location;
      origin = type === "REPLACE" ? undefined : { pathname, search, hash, workspace: seen.workspace };
    }
    setSeen({ location, workspace: slug, settings, origin });
  }
  return seen.origin;
}

/** SettingsOrigin is the origin Layout recorded (useOriginRecord): the directory's Back goes there. */
export const SettingsOrigin = createContext<Origin | undefined>(undefined);

/** useSettingsOrigin is where the settings shown were entered from; undefined, opened at their address. */
export function useSettingsOrigin(): Origin | undefined {
  return use(SettingsOrigin);
}
