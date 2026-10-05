import { useRef } from "react";

import type { HeldDialog } from "../../app/held-dialog";
import { notebookNameProblem, notebookNameTexts } from "../../app/notebook-name";
import { pagesLocked } from "../../app/pages-locked";
import { RenameForm } from "../../app/rename-form";
import { Dialog, DialogContent, DialogTitle } from "../../components/ui/dialog";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";

/**
 * RenamePageDialog renames page with the rename form a notebook's has: a
 * page's title follows a notebook name's rules (shared.CheckTitle). A
 * refusal (422, 409 page.title_taken) stays in the form, one for the pages
 * whose links the rename would write again being edited with those pages
 * and their editors (M6/P4); once saved, the dialog closes. Its title names
 * the page by name, which tells it from others of its title.
 */
export function RenamePageDialog({
  notebook,
  page,
  name,
  held,
}: {
  notebook: Notebook;
  page: TreeNode;
  name: string;
  held: HeldDialog;
}) {
  const pages = usePageTree(notebook);
  const t = useT();
  const me = useStore().account?.me?.id;
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
          <DialogTitle>{t("page.renameTitle", { name })}</DialogTitle>
          <RenameForm
            current={page.name}
            label={t("page.title")}
            autoComplete="off"
            check={notebookNameProblem}
            fieldTexts={notebookNameTexts}
            rename={(title) => pages.rename(page.id, title)}
            explain={(error) => pagesLocked(error, t, me, (id) => pages.byId(id)?.name)}
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
