import { Ellipsis } from "lucide-react";
import { useRef, useState } from "react";
import { useNavigate } from "react-router";

import { Button } from "../../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../../components/ui/dropdown-menu";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { ExportDialog } from "../notebook/export-dialog";
import type { FocusJob } from "../notebook/transfer-page";

/**
 * PageMenu is the menu beside a page's title (M7/P5 design 4.5), which
 * readers have too: the tree's menu is its writers'. Export this page
 * exports it with its subtree, then goes to the notebook's imports and
 * exports, the job's row taking the focus; cancelled, the focus goes back
 * to the menu.
 */
export function PageMenu({ notebook, page, href }: { notebook: Notebook; page: TreeNode; href: string }) {
  const t = useT();
  const navigate = useNavigate();
  const [exporting, setExporting] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const started = useRef<string | undefined>(undefined);
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button ref={trigger} variant="outline" size="icon" aria-label={t("page.menu")}>
            <Ellipsis />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={() => setExporting(true)}>{t("page.export")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ExportDialog
        notebook={notebook}
        page={page}
        held={{
          open: exporting,
          onOpenChange: setExporting,
          onClosed: (done) => {
            const job = started.current;
            started.current = undefined;
            if (done && job !== undefined) {
              void navigate(href, { state: { focusJob: job } satisfies FocusJob });
            } else {
              trigger.current?.focus();
            }
          },
        }}
        onStarted={(job) => (started.current = job.id)}
      />
    </>
  );
}
