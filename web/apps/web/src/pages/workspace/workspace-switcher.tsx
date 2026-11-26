import { ChevronsUpDown, Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useId } from "react";
import { Link, useNavigate } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { initialOf } from "../../app/initial";
import { Button } from "../../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../../components/ui/dropdown-menu";
import { useT } from "../../i18n/i18n";
import type { Workspace } from "../../services/workspace.service";
import { useStore, useWorkspaces } from "../../stores/context";

/**
 * WorkspaceSwitcher is the workspace shown, and the way to the account's
 * others: choosing one goes to it. While this server lets accounts create
 * workspaces, it is also the way to create one (M2/P5 design 3.2), arrived
 * at: the switcher leaves with the workspace's shell (M13/P2/S1, decision
 * 771). Its button is the workspace's name after a square avatar of its
 * initial, which only the eye reads (M13/P2/S1). A name cut short has its
 * whole as its title, on its text (M13 closeout, review A's M13CA-M1); the
 * menu is as wide as the button at least, wider for a long name, up to 20rem
 * and the room it has (decision 971).
 */
export const WorkspaceSwitcher = observer(function WorkspaceSwitcher({ current }: { current: Workspace }) {
  const workspaces = useWorkspaces();
  const { instance } = useStore();
  const t = useT();
  const navigate = useNavigate();
  const description = useId();
  useSWR("instance", () => instance.load());
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          className="h-9 w-full justify-start gap-2.25 rounded-row px-2 text-sm font-semibold text-foreground [&_svg]:size-3.5"
          aria-describedby={description}
        >
          <span
            aria-hidden
            className="grid size-5.5 shrink-0 place-items-center rounded-md bg-primary text-xs font-semibold text-primary-foreground"
          >
            {initialOf(current.name)}
          </span>
          <span title={current.name} className="min-w-0 truncate">
            {current.name}
          </span>
          <ChevronsUpDown className="ml-auto text-faint" />
        </Button>
      </DropdownMenuTrigger>
      <span id={description} hidden>
        {t("workspace.switch")}
      </span>
      <DropdownMenuContent
        align="start"
        // 8px off the window's edges, as the account's submenu: at 320px it ran from one to the other, over the drawer's
        // shade (M13 closeout fix check 1, review A's M13CA-F1-N1).
        collisionPadding={8}
        className="max-w-[min(20rem,var(--radix-dropdown-menu-content-available-width))] min-w-(--radix-dropdown-menu-trigger-width)"
      >
        <DropdownMenuRadioGroup value={current.slug} onValueChange={(slug) => void navigate(`/${slug}`)}>
          {workspaces.list?.map((workspace) => (
            // Two lines at most, the item as high as they are: cut short on one line, two names that differ at
            // their end alone read the same at 320px, where no pointer hovers for the title (M13 closeout fix check 2,
            // review A's M13CA-F2-N2). The button keeps its one line.
            <DropdownMenuRadioItem key={workspace.id} value={workspace.slug} className="h-auto min-h-8 py-1.5">
              <span title={workspace.name} className="line-clamp-2 min-w-0 [overflow-wrap:anywhere]">
                {workspace.name}
              </span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
        {instance.info?.workspace_creation_enabled === true && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link to="/create-workspace" state={arrived}>
                <Plus />
                {t("workspace.create")}
              </Link>
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
});
