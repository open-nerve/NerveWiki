import { ChevronsUpDown, Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { Link, useNavigate } from "react-router";
import useSWR from "swr";

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
 * workspaces, it is also the way to create one (M2/P5 design 3.2).
 */
export const WorkspaceSwitcher = observer(function WorkspaceSwitcher({ current }: { current: Workspace }) {
  const workspaces = useWorkspaces();
  const { instance } = useStore();
  const t = useT();
  const navigate = useNavigate();
  useSWR("instance", () => instance.load());
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="w-full justify-between">
          <span className="truncate">{current.name}</span>
          <ChevronsUpDown />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-56">
        <DropdownMenuRadioGroup value={current.slug} onValueChange={(slug) => void navigate(`/${slug}`)}>
          {workspaces.list?.map((workspace) => (
            <DropdownMenuRadioItem key={workspace.id} value={workspace.slug}>
              <span className="truncate">{workspace.name}</span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
        {instance.info?.workspace_creation_enabled === true && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link to="/create-workspace">
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
