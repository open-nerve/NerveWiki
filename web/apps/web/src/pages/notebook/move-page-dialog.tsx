import { observer } from "mobx-react-lite";
import { useId, useRef, useState, type FormEvent } from "react";

import { useForm } from "../../app/form";
import type { HeldDialog } from "../../app/held-dialog";
import { pagesLocked } from "../../app/pages-locked";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "../../components/ui/dialog";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { NodeMove, TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { canHold, childrenOf } from "../../stores/page-tree";
import { distinctName } from "./distinct-name";
import { ParentOptions, Problem, rootOption as root } from "./parent-options";

/**
 * MovePageDialog moves page, with the pages under it, by two selects
 * (M4/P5 design 3.7): the keyboard's way to what dragging does (WCAG 2.2,
 * 2.5.7). The parents offered are those that can hold it: not the page
 * itself nor a page under it, none it would go deeper than ten levels
 * under; the positions are first, after each sibling, last. It opens on
 * the page's place; a choice the tree, read again, no longer offers goes
 * back to the page's parent and last. It is sent as every form is
 * (useForm): a refusal (409 page.cycle, page.too_deep, page.title_taken)
 * stays in the dialog, above the form, one for the pages whose links the
 * move would write again being edited with those pages, named as the tree
 * names them, and their editors (M6/P4), one of a field's (422, a parent
 * or a page to follow gone meanwhile) under its select, which gets the
 * focus; once moved, the dialog closes. onSend is called as it sends. Its
 * title names the page by name, which tells it from others of its title.
 */
export function MovePageDialog({
  notebook,
  page,
  name,
  held,
  onSend,
}: {
  notebook: Notebook;
  page: TreeNode;
  name: string;
  held: HeldDialog;
  onSend: () => void;
}) {
  const t = useT();
  const done = useRef(false);
  return (
    <Dialog open={held.open} onOpenChange={held.onOpenChange}>
      {held.open && (
        <DialogContent
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            held.onClosed(done.current);
            done.current = false;
          }}
        >
          <DialogTitle>{t("page.moveTitle", { name })}</DialogTitle>
          <DialogDescription>{t("page.moveBody")}</DialogDescription>
          <MoveForm
            notebook={notebook}
            page={page}
            onSend={onSend}
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

type MoveFormProps = { notebook: Notebook; page: TreeNode; onSend: () => void; cancel: () => void; moved: () => void };

const MoveForm = observer(function MoveForm({ notebook, page, onSend, cancel, moved }: MoveFormProps) {
  const pages = usePageTree(notebook);
  const t = useT();
  const ids = { parent: useId(), position: useId() };
  const [parent, setParent] = useState(page.parent_id ?? root);
  const [position, setPosition] = useState(() => placeOf(pages.childrenOf(page.parent_id), page.id));
  const me = useStore().account?.me?.id;
  const { ref, sending, banner, problemOf, submit } = useForm(["parent_id", "after_id"], {
    explain: (error) => pagesLocked(error, t, me, (id) => distinctName(pages.tree, notebook, id, t)),
  });
  const problems = { parent: problemOf("parent_id"), position: problemOf("after_id") };
  const tree = pages.tree;
  if (tree === undefined) {
    return null;
  }
  const parents = [...tree.byId.values()].filter((each) => canHold(tree, each.id, page.id));
  // A choice that the tree, read again, no longer offers is the default again: what the selects show goes out.
  const offered = (id: string) => id === root || parents.some((each) => each.id === id);
  const chosen = offered(parent) ? parent : offered(page.parent_id ?? root) ? (page.parent_id ?? root) : root;
  const siblings = childrenOf(tree, chosen === root ? null : chosen).filter((each) => each.id !== page.id);
  const place =
    position === "first" || position === "last" || siblings.some((each) => each.id === position) ? position : "last";

  function choose(next: string) {
    setParent(next);
    setPosition(
      next === (page.parent_id ?? root) && tree !== undefined
        ? placeOf(childrenOf(tree, page.parent_id), page.id)
        : "last"
    );
  }

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const parentId = chosen === root ? null : chosen;
    const move: NodeMove =
      place === "last" ? { parent_id: parentId } : { parent_id: parentId, after_id: place === "first" ? null : place };
    void submit({}, async () => {
      onSend();
      await pages.move(page.id, move);
      pages.openTo(page.id);
      moved();
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <div className="space-y-2">
        <Label htmlFor={ids.parent}>{t("page.moveParent")}</Label>
        <NativeSelect
          id={ids.parent}
          value={chosen}
          aria-invalid={problems.parent !== undefined || undefined}
          aria-describedby={problems.parent === undefined ? undefined : `${ids.parent}-note`}
          onChange={(event) => choose(event.target.value)}
        >
          <ParentOptions pages={pages} parents={parents} />
        </NativeSelect>
        <Problem id={`${ids.parent}-note`} text={problems.parent} />
      </div>
      <div className="space-y-2">
        <Label htmlFor={ids.position}>{t("page.movePosition")}</Label>
        <NativeSelect
          id={ids.position}
          value={place}
          aria-invalid={problems.position !== undefined || undefined}
          aria-describedby={problems.position === undefined ? undefined : `${ids.position}-note`}
          onChange={(event) => setPosition(event.target.value)}
        >
          <option value="first">{t("page.moveFirst")}</option>
          {siblings.map((sibling) => (
            <option key={sibling.id} value={sibling.id}>
              {t("page.moveAfter", { name: sibling.name })}
            </option>
          ))}
          <option value="last">{t("page.moveLast")}</option>
        </NativeSelect>
        <Problem id={`${ids.position}-note`} text={problems.position} />
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

/** placeOf is where the page id is among its siblings, as the position's select says it: after the one before, or first. */
function placeOf(siblings: readonly TreeNode[], id: string): string {
  return siblings[siblings.findIndex((sibling) => sibling.id === id) - 1]?.id ?? "first";
}
