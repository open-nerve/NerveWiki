import { createContext, use, type ReactNode, type RefObject } from "react";

import { useArrivalFocus } from "./arrival";

/** The shown settings page's h1, which a row removed with the focus in it gives the focus to (decision 815). */
const SettingsHeading = createContext<RefObject<HTMLHeadingElement | null> | undefined>(undefined);

/** useSettingsHeading is the h1 of the settings page shown: only below SettingsPage. */
export function useSettingsHeading(): RefObject<HTMLHeadingElement | null> {
  const heading = use(SettingsHeading);
  if (heading === undefined) {
    throw new Error("useSettingsHeading is used outside SettingsPage");
  }
  return heading;
}

/**
 * SettingsPage is a page of the settings' mode (00-M13 4.4.2; M13/P3/S1, decisions 815, 820, 844): its name as its h1,
 * which takes the arrival focus as it mounts (the layouts key it by the page: arrived at twice in one layout, from the
 * drawer, each page's h1 mounts anew and takes it), above its groups. Centred, its content at most 696px wide (the
 * mockup's .page, 760px with 32px each side), the main's 24px of padding made the mockup's 44 / 32 / 40 from md: up
 * (P4 decides the main's own).
 */
export function SettingsPage({ title, children }: { title: string; children: ReactNode }) {
  const heading = useArrivalFocus<HTMLHeadingElement>();
  return (
    <div className="mx-auto w-full max-w-178 md:px-2 md:pt-5 md:pb-4">
      <h1
        ref={heading}
        tabIndex={-1}
        className="mb-7 text-[26px] leading-normal font-[650] tracking-[-0.01em] break-words outline-none"
      >
        {title}
      </h1>
      <SettingsHeading value={heading}>
        <div className="space-y-6.5">{children}</div>
      </SettingsHeading>
    </div>
  );
}
