import { CircleUser, LogOut, Settings } from "lucide-react";
import { observer } from "mobx-react-lite";
import { Link } from "react-router";
import useSWR from "swr";

import { Button } from "../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import { useT } from "../i18n/i18n";
import { useStore } from "../stores/context";

/**
 * UserMenu is the signed-in account in the top bar: its display name, the
 * way to the settings, and signing out, which ends the session in every
 * tab of the browser; each tab's guard then takes it to the sign-in page
 * (M1/P5 design 3.6). Its last line is the version this server runs (M2/P5
 * design 3.8).
 */
export const UserMenu = observer(function UserMenu() {
  const { account, auth, instance } = useStore();
  const t = useT();
  const me = account?.me;
  useSWR(me === undefined ? null : "instance", () => instance.load());
  const info = instance.info;
  if (me === undefined) {
    return null;
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost">
          <CircleUser />
          <span className="max-w-40 truncate">{me.display_name}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem asChild>
          <Link to="/settings/profile">
            <Settings />
            {t("userMenu.settings")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => void auth.signOut()}>
          <LogOut />
          {t("userMenu.signOut")}
        </DropdownMenuItem>
        {info !== undefined && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel>
              {t("userMenu.version", { version: info.version, commit: info.commit })}
            </DropdownMenuLabel>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
});
