import { ChevronDown } from "lucide-react";
import { useState } from "react";

import { Button } from "../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import { useT } from "../i18n/i18n";

type RoleMenuProps<Role extends string> = {
  role: Role;
  /** The roles to choose from, from the most rights to the fewest. */
  roles: readonly Role[];
  /** The name of each role. */
  label: (role: Role) => string;
  /** Whose role it is, as the button names them. */
  who: string;
  /** Changes the role; a refusal is the list's to say. */
  changeRole: (role: Role) => Promise<void>;
};

/**
 * RoleMenu changes a member's role as one is chosen: a menu, so that
 * moving through the roles chooses none, and the focus goes back to its
 * button. One change goes out at a time; a choice made while one is out
 * is not taken. A workspace's members and a notebook's have one each
 * (M3/P4 design 3.4).
 */
export function RoleMenu<Role extends string>({ role, roles, label, who, changeRole }: RoleMenuProps<Role>) {
  const t = useT();
  const [sending, setSending] = useState(false);

  async function choose(chosen: Role) {
    if (sending || chosen === role) {
      return;
    }
    setSending(true);
    try {
      await changeRole(chosen);
    } finally {
      setSending(false);
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          aria-label={t("members.roleOf", { role: label(role), name: who })}
          aria-busy={sending || undefined}
        >
          {label(role)}
          <ChevronDown />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup
          value={role}
          onValueChange={(chosen) => void choose(roles.find((each) => each === chosen) ?? role)}
        >
          {roles.map((each) => (
            <DropdownMenuRadioItem key={each} value={each}>
              {label(each)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
