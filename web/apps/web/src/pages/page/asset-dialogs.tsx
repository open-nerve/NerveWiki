import { observer } from "mobx-react-lite";
import { useId, useRef, useState, type FormEvent } from "react";

import { useForm } from "../../app/form";
import type { HeldDialog } from "../../app/held-dialog";
import { notebookNameProblem, notebookNameTexts } from "../../app/notebook-name";
import { pagesLocked } from "../../app/pages-locked";
import { RenameForm } from "../../app/rename-form";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "../../components/ui/dialog";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { useT } from "../../i18n/i18n";
import { extensionOf } from "../../lib/upload-name";
import type { Asset } from "../../services/asset.service";
import type { Notebook } from "../../services/notebook.service";
import { useAssets, usePageTree, useStore } from "../../stores/context";
import { distinctName } from "../notebook/distinct-name";
import { ParentOptions, Problem, rootOption as root } from "../notebook/parent-options";

type AssetDialogProps = { notebook: Notebook; asset: Asset; held: HeldDialog };

/**
 * RenameAssetDialog renames an attachment by its stem (M7/P4 design 3.5):
 * its extension shows after the field, and stays, as a rename of an
 * attachment keeps it (M7 design 4.2); one without an extension is renamed
 * whole. The name follows a title's rules, as a page's does; a refusal
 * stays in the form, one for the pages whose links the rename would write
 * again being edited with those pages and their editors; once saved, the
 * dialog closes.
 */
export function RenameAssetDialog({ notebook, asset, held }: AssetDialogProps) {
  const assets = useAssets(notebook);
  const pages = usePageTree(notebook);
  const t = useT();
  const me = useStore().account?.me?.id;
  const done = useRef(false);
  const extension = extensionOf(asset.name);
  const stem = asset.name.slice(0, asset.name.length - extension.length);
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
          <DialogTitle>{t("asset.renameTitle", { name: asset.name })}</DialogTitle>
          <RenameForm
            current={stem}
            label={t("asset.name")}
            hint={extension === "" ? undefined : t("asset.renameKeeps", { extension })}
            suffix={extension === "" ? undefined : extension}
            autoComplete="off"
            check={(name) => (name.trim() === "" ? "field.required" : notebookNameProblem(name.trim() + extension))}
            fieldTexts={notebookNameTexts}
            rename={(name) => assets.rename(asset.id, asset.parent_id, name + extension)}
            explain={(error) => pagesLocked(error, t, me, (id) => distinctName(pages.tree, notebook, id, t))}
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

/**
 * MoveAssetDialog moves an attachment under another page, or to the
 * notebook's top level (M7/P4 design 3.5), where the list has it by its
 * name: the parents' select is a page's move's, of every page, as any
 * page holds attachments, with no position. Its own parent sends nothing.
 * A refusal stays in the dialog, as a page's move's does; once moved, the
 * dialog closes.
 */
export function MoveAssetDialog({ notebook, asset, held }: AssetDialogProps) {
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
          <DialogTitle>{t("asset.moveTitle", { name: asset.name })}</DialogTitle>
          <MoveAssetForm
            notebook={notebook}
            asset={asset}
            cancel={() => held.onOpenChange(false)}
            moved={() => {
              done.current = true;
              held.onOpenChange(false);
            }}
          />
        </DialogContent>
      )}
    </Dialog>
  );
}

type MoveAssetFormProps = { notebook: Notebook; asset: Asset; cancel: () => void; moved: () => void };

const MoveAssetForm = observer(function MoveAssetForm({ notebook, asset, cancel, moved }: MoveAssetFormProps) {
  const assets = useAssets(notebook);
  const pages = usePageTree(notebook);
  const t = useT();
  const id = useId();
  const [parent, setParent] = useState(asset.parent_id ?? root);
  const me = useStore().account?.me?.id;
  const { ref, sending, banner, problemOf, submit } = useForm(["parent_id"], {
    explain: (error) => pagesLocked(error, t, me, (each) => distinctName(pages.tree, notebook, each, t)),
  });
  const problem = problemOf("parent_id");
  const tree = pages.tree;
  if (tree === undefined) {
    return null;
  }
  const parents = [...tree.byId.values()];
  // A page gone since it was chosen is the attachment's parent again: what the select shows goes out.
  const offered = (each: string) => each === root || tree.byId.has(each);
  const chosen = offered(parent) ? parent : offered(asset.parent_id ?? root) ? (asset.parent_id ?? root) : root;

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const to = chosen === root ? null : chosen;
    void submit({}, async () => {
      if (to !== asset.parent_id) {
        await assets.move(asset.id, asset.parent_id, to);
      }
      moved();
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <div className="space-y-2">
        <Label htmlFor={id}>{t("page.moveParent")}</Label>
        <NativeSelect
          id={id}
          value={chosen}
          aria-invalid={problem !== undefined || undefined}
          aria-describedby={problem === undefined ? undefined : `${id}-note`}
          onChange={(event) => setParent(event.target.value)}
        >
          <ParentOptions pages={pages} parents={parents} />
        </NativeSelect>
        <Problem id={`${id}-note`} text={problem} />
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={cancel} disabled={sending}>
          {t("page.cancel")}
        </Button>
        <Button type="submit" disabled={sending}>
          {sending ? t("page.moving") : t("page.move")}
        </Button>
      </div>
    </form>
  );
});
