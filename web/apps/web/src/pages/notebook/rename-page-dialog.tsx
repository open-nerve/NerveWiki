import { useRef } from "react";

import { notebookNameProblem, notebookNameTexts } from "../../app/notebook-name";
import { RenameForm } from "../../app/rename-form";
import { Dialog, DialogContent, DialogTitle } from "../../components/ui/dialog";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";

/** The dialog held by its page's menu: whether it is open, the change it asks for, and what follows its closing. */
export type HeldDialog = { open: boolean; onOpenChange: (open: boolean) => void; onClosed: (done: boolean) => void };

/**
 * RenamePageDialog renames page with the rename form a notebook's has: a
 * page's title follows a notebook name's rules (shared.CheckTitle). A
 * refusal (422, 409 page.title_taken) stays in the form; once saved, the
 * dialog closes.
 */
export function RenamePageDialog({ notebook, page, held }: { notebook: Notebook; page: TreeNode; held: HeldDialog }) {
  const pages = usePageTree(notebook);
  const t = useT();
  const done = useRef(false);
  return (
    <Dialog open={held.open} onOpenChange={held.onOpenChange}>
      {held.open && (
        <DialogContent
          aria-describedby={undefined}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            held.onClosed(done.current);
            done.current = false;
          }}
        >
          <DialogTitle>{t("page.renameTitle", { name: page.name })}</DialogTitle>
          <RenameForm
            current={page.name}
            label={t("page.title")}
            autoComplete="off"
            check={notebookNameProblem}
            fieldTexts={notebookNameTexts}
            rename={(name) => pages.rename(page.id, name)}
            saveLabel={t("page.save")}
            savedLabel={t("page.saved")}
            onSaved={() => {
              done.current = true;
              held.onOpenChange(false);
            }}
          />
        </DialogContent>
      )}
    </Dialog>
  );
}
