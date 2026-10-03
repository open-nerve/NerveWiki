import { observer } from "mobx-react-lite";
import { useEffect, useId, useState, type KeyboardEvent } from "react";
import { useNavigate } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { NotLoaded } from "../../app/not-loaded";
import { isMod, onMac } from "../../app/shortcuts";
import { Dialog, DialogContent, DialogTitle } from "../../components/ui/dialog";
import { Input } from "../../components/ui/input";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import { ancestorsOf, findPages } from "../../stores/page-tree";
import { useWorkspace } from "../workspace/workspace-layout";

/** The most pages the quick switch lists (M4/P5 design 3.10). */
const shown = 50;

/**
 * QuickSwitch goes to a page of notebook by its title (M4/P5 design 3.10).
 * Mod+O (Cmd+O on macOS, Ctrl+O elsewhere) opens it while the notebook is
 * open, instead of the browser's Open File.
 */
export function QuickSwitch({ notebook }: { notebook: Notebook }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const mac = onMac();
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (isMod(event, "o", mac)) {
        event.preventDefault();
        setOpen(true);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {open && (
        <DialogContent aria-describedby={undefined}>
          <DialogTitle>{t("page.quickSwitch")}</DialogTitle>
          <Finder notebook={notebook} close={() => setOpen(false)} />
        </DialogContent>
      )}
    </Dialog>
  );
}

/**
 * Finder is the quick switch's field and the pages whose title holds what
 * is typed, each with its ancestors, the first 50: a combobox and its
 * listbox, Up and Down to move, Enter to go. Until the tree is read it
 * says so, with Try again.
 */
const Finder = observer(function Finder({ notebook, close }: { notebook: Notebook; close: () => void }) {
  const pages = usePageTree(notebook);
  const { slug } = useWorkspace();
  const navigate = useNavigate();
  const t = useT();
  const ids = { list: useId(), option: useId() };
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const tree = pages.tree;
  const optionId = (index: number) => `${ids.option}-${index}`;
  useEffect(() => {
    document.getElementById(optionId(active))?.scrollIntoView?.({ block: "nearest" });
  });
  if (tree === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const found = findPages(tree, query);
  const listed = found.slice(0, shown);

  function go(id: string) {
    close();
    void navigate(`/${slug}/notebooks/${notebook.id}/pages/${id}`, { state: arrived });
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive(Math.min(Math.max(active + step, 0), Math.max(listed.length - 1, 0)));
    } else if (event.key === "Enter") {
      event.preventDefault();
      const page = listed[active];
      if (page !== undefined) {
        go(page.id);
      }
    }
  }

  return (
    <div className="space-y-3">
      <Input
        // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- ARIA's combobox: the field keeps the focus and names the active option
        role="combobox"
        aria-label={t("page.quickSwitchField")}
        aria-expanded={listed.length > 0}
        aria-controls={ids.list}
        aria-autocomplete="list"
        aria-activedescendant={listed[active] === undefined ? undefined : optionId(active)}
        autoComplete="off"
        value={query}
        onChange={(event) => {
          setQuery(event.target.value);
          setActive(0);
        }}
        onKeyDown={onKeyDown}
      />
      {listed.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("page.quickSwitchNone")}</p>
      ) : (
        <ul
          id={ids.list}
          // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role, jsx-a11y/no-noninteractive-element-to-interactive-role -- the combobox's listbox: a select's options cannot show their ancestors
          role="listbox"
          aria-label={t("page.quickSwitchPages")}
          className="max-h-80 overflow-y-auto rounded-md border"
        >
          {listed.map((page, index) => (
            // oxlint-disable-next-line jsx-a11y/click-events-have-key-events -- the keys go to the combobox, which names the active option
            <li
              key={page.id}
              id={optionId(index)}
              // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role, jsx-a11y/no-noninteractive-element-to-interactive-role -- an option of the listbox above
              role="option"
              aria-selected={index === active}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => go(page.id)}
              className={cn("cursor-pointer px-3 py-1.5 text-sm", index === active && "bg-accent")}
            >
              <span className="block truncate">{page.name}</span>{" "}
              <span className="block truncate text-xs text-muted-foreground">
                {ancestorsOf(tree, page.id)
                  .map((ancestor) => ancestor.name)
                  .join(" / ")}
              </span>
            </li>
          ))}
        </ul>
      )}
      {found.length > shown && (
        <p className="text-xs text-muted-foreground">{t("page.quickSwitchFirst", { count: shown })}</p>
      )}
    </div>
  );
});
