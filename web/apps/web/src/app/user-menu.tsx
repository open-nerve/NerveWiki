import { Ellipsis, LogOut, Settings, SunMoon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useRef } from "react";
import { Link, useMatch } from "react-router";
import useSWR from "swr";

import { Button } from "../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import { useT } from "../i18n/i18n";
import { pendingSteps } from "../onboarding/steps";
import { useStore } from "../stores/context";
import { AppearanceItems } from "./appearance-menu";
import { arrived } from "./arrival";
import { initialOf } from "./initial";
import { useNarrow } from "./narrow";
import { useSignOut } from "./sign-out";

/**
 * UserMenu is the signed-in account (M13/P2/S1, decision 770): in the left
 * column's foot (row), or at the top right where no left column is
 * (corner); one of them shows at a time. Its button is the display name
 * after an avatar of its initial, which only the eye reads. Its menu: the
 * settings, arrived at, as the column leaves with the workspace's shell
 * (decision 771); Appearance, the theme and the language chosen in place,
 * a submenu, below md in the menu itself: beside a menu as wide as a
 * phone's column a submenu has no room, and Radix opens it to the right
 * all the same (decision 801); signing out, which ends the session in
 * every tab of the browser, each tab's guard then taking it to the sign-in
 * page (M1/P5 design 3.6), asking first while an edit has changes not
 * saved (useSignOut), Stay giving the focus back to the menu's button;
 * last, the version this server runs (M2/P5 design 3.8).
 */
export const UserMenu = observer(function UserMenu({ place }: { place: "row" | "corner" }) {
  const { account, instance } = useStore();
  const t = useT();
  const me = account?.me;
  useSWR(me === undefined ? null : "instance", () => instance.load());
  const trigger = useRef<HTMLButtonElement>(null);
  const { signOut, dialog } = useSignOut(() => trigger.current?.focus());
  // The settings chosen, the focus is their title's: the menu is not to give it back to its button, which stays in
  // the column's foot on the settings (page-tree-item.tsx's chosen). Chosen on the profile itself, the address is the
  // same, the title taking nothing: the button gets it back (review A, P2A1-N1); so too with a step of onboarding
  // left, the settings sending it back there (review A, S1A-N2). On the profile itself no arrival either, the link
  // replacing the address shown: below md the drawer then gives its menu the focus (M13/P3/S1, decision 816).
  const leaving = useRef(false);
  const atProfile = useMatch("/settings/profile") !== null;
  const narrow = useNarrow();
  const info = instance.info;
  if (me === undefined) {
    return null;
  }
  const name = me.display_name;
  const row = place === "row";
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          {row ? (
            <Button
              ref={trigger}
              variant="ghost"
              className="h-10 w-full justify-start gap-2.25 rounded-row px-2 text-row font-normal text-foreground"
            >
              <span
                aria-hidden
                className="grid size-6 shrink-0 place-items-center rounded-full bg-avatar text-[11px] font-semibold text-avatar-foreground"
              >
                {initialOf(name)}
              </span>
              <span className="min-w-0 truncate">{name}</span>
              <Ellipsis className="ml-auto text-faint" />
            </Button>
          ) : (
            <Button
              ref={trigger}
              variant="ghost"
              size="sm"
              shape="pill"
              className="min-w-0 shrink gap-2 pl-1.5 text-foreground"
            >
              <span
                aria-hidden
                className="grid size-5.5 shrink-0 place-items-center rounded-full bg-avatar text-[11px] font-semibold text-avatar-foreground"
              >
                {initialOf(name)}
              </span>
              <span className="max-w-32 min-w-0 truncate">{name}</span>
            </Button>
          )}
        </DropdownMenuTrigger>
        <DropdownMenuContent
          side={row ? "top" : undefined}
          align={row ? "start" : "end"}
          // The top right's menu is as wide as its longest line, the version, but no wider than 16rem, near the
          // column's (review B, S1B-N3), nor than the screen less the corner's own gutter below md (px-3): a phone has
          // less room than the 40 digits of the commit.
          collisionPadding={row ? undefined : 12}
          className={
            row
              ? "w-(--radix-dropdown-menu-trigger-width)"
              : "max-w-[min(16rem,var(--radix-dropdown-menu-content-available-width))]"
          }
          onCloseAutoFocus={(event) => {
            if (leaving.current) {
              event.preventDefault();
            }
            leaving.current = false;
          }}
        >
          <DropdownMenuItem
            asChild
            onSelect={() => {
              leaving.current = !atProfile && pendingSteps(me).length === 0;
            }}
          >
            <Link to="/settings/profile" state={atProfile ? undefined : arrived}>
              <Settings />
              {t("userMenu.settings")}
            </Link>
          </DropdownMenuItem>
          {narrow ? (
            <>
              <DropdownMenuSeparator />
              <AppearanceItems />
            </>
          ) : (
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <SunMoon />
                {t("appearance.label")}
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <AppearanceItems />
              </DropdownMenuSubContent>
            </DropdownMenuSub>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => signOut()}>
            <LogOut />
            {t("userMenu.signOut")}
          </DropdownMenuItem>
          {info !== undefined && (
            <>
              <DropdownMenuSeparator />
              {/* The row's menu is as wide as the row, the top right's 16rem at most: the commit, 40 digits, wraps
                  where it must. */}
              <DropdownMenuLabel className="wrap-anywhere">
                {t("userMenu.version", { version: info.version, commit: info.commit })}
              </DropdownMenuLabel>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      {dialog}
    </>
  );
});
