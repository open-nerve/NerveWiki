import { observer } from "mobx-react-lite";
import { useId, type ReactNode } from "react";
import useSWR from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks } from "../../stores/context";
import { groupNotebooks } from "../../stores/notebook.store";

type NotebookGroupsProps = {
  workspace: Workspace;
  /** Each notebook's entry in its group's list. */
  renderItem: (notebook: Notebook) => ReactNode;
  /** What shows when the account sees no notebook. */
  empty: ReactNode;
  headingClassName: string;
  listClassName: string;
};

/**
 * NotebookGroups are the notebooks of workspace that the account sees, in
 * their two groups (M3/P4 design 3.3): each a heading and the list it
 * names, empty ones not shown. The left column and the workspace's home
 * show them from one read, the notebook pages' too (SWR's key
 * ["notebooks", id]).
 */
export const NotebookGroups = observer(function NotebookGroups({
  workspace,
  renderItem,
  empty,
  headingClassName,
  listClassName,
}: NotebookGroupsProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const { error, mutate } = useSWR(["notebooks", workspace.id], () => notebooks.load());
  if (notebooks.list === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const { mine, team } = groupNotebooks(notebooks.list);
  const groups = [
    { title: t("notebooks.mine"), list: mine },
    { title: t("notebooks.team"), list: team },
  ].filter((group) => group.list.length > 0);
  if (groups.length === 0) {
    return empty;
  }
  return groups.map(({ title, list }) => (
    <Group key={title} title={title} headingClassName={headingClassName} listClassName={listClassName}>
      {list.map((notebook) => (
        <li key={notebook.id} className="flex flex-col">
          {renderItem(notebook)}
        </li>
      ))}
    </Group>
  ));
});

type GroupProps = { title: string; headingClassName: string; listClassName: string; children: ReactNode };

function Group({ title, headingClassName, listClassName, children }: GroupProps) {
  const id = useId();
  return (
    <div className="space-y-2">
      <h2 id={id} className={headingClassName}>
        {title}
      </h2>
      <ul aria-labelledby={id} className={listClassName}>
        {children}
      </ul>
    </div>
  );
}
