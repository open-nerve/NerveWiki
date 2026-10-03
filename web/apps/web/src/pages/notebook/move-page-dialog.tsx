import { observer } from "mobx-react-lite";
import { useId, useRef, useState, type FormEvent } from "react";

import type { HeldDialog } from "../../app/held-dialog";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "../../components/ui/dialog";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { NodeMove, TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import { canHold, childrenOf } from "../../stores/page-tree";

/** The root's value in the parent's select. */
const root = "";

/**
 * MovePageDialog moves page, with the pages under it, by two selects
 * (M4/P5 design 3.7): the keyboard's way to what dragging does (WCAG 2.2,
 * 2.5.7). The parents offered are those that can hold it: not the page
 * itself nor a page under it, none it would go deeper than ten levels
 * under; the positions are first, after each sibling, last. It opens on
 * the page's place; a choice the tree, read again, no longer offers goes
 * back to the page's parent and last. A refusal (409 page.cycle,
 * page.too_deep, page.title_taken) stays in the dialog; once moved, the
 * dialog closes.
 */
export function MovePageDialog({ notebook, page, held }: { notebook: Notebook; page: TreeNode; held: HeldDialog }) {
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
          <DialogTitle>{t("page.moveTitle", { name: page.name })}</DialogTitle>
          <DialogDescription>{t("page.moveBody")}</DialogDescription>
          <MoveForm
            notebook={notebook}
            page={page}
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

type MoveFormProps = { notebook: Notebook; page: TreeNode; cancel: () => void; moved: () => void };

const MoveForm = observer(function MoveForm({ notebook, page, cancel, moved }: MoveFormProps) {
  const pages = usePageTree(notebook);
  const t = useT();
  const ids = { parent: useId(), position: useId() };
  const [parent, setParent] = useState(page.parent_id ?? root);
  const [position, setPosition] = useState(() => placeOf(pages.childrenOf(page.parent_id), page.id));
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState<unknown>();
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

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const parentId = chosen === root ? null : chosen;
    const move: NodeMove =
      place === "last" ? { parent_id: parentId } : { parent_id: parentId, after_id: place === "first" ? null : place };
    setSending(true);
    setFailure(undefined);
    try {
      await pages.move(page.id, move);
    } catch (error) {
      setFailure(error);
      setSending(false);
      return;
    }
    pages.openTo(page.id);
    moved();
  }

  return (
    <form noValidate onSubmit={(event) => void onSubmit(event)} className="space-y-4">
      {failure !== undefined && <Alert>{errorText(failure, t)}</Alert>}
      <div className="space-y-2">
        <Label htmlFor={ids.parent}>{t("page.moveParent")}</Label>
        <NativeSelect id={ids.parent} value={chosen} onChange={(event) => choose(event.target.value)}>
          <option value={root}>{t("page.moveRoot")}</option>
          {parents.map((each) => (
            <option key={each.id} value={each.id}>
              {[...pages.ancestorsOf(each.id), each].map((step) => step.name).join(" / ")}
            </option>
          ))}
        </NativeSelect>
      </div>
      <div className="space-y-2">
        <Label htmlFor={ids.position}>{t("page.movePosition")}</Label>
        <NativeSelect id={ids.position} value={place} onChange={(event) => setPosition(event.target.value)}>
          <option value="first">{t("page.moveFirst")}</option>
          {siblings.map((sibling) => (
            <option key={sibling.id} value={sibling.id}>
              {t("page.moveAfter", { name: sibling.name })}
            </option>
          ))}
          <option value="last">{t("page.moveLast")}</option>
        </NativeSelect>
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
