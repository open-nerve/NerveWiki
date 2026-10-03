import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import { useNavigate, useParams } from "react-router";
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
 * open, instead of the browser's Open File; not over another dialog.
 * Closed, it gives the focus back to where it was, unless it went to
 * another page: that page's heading takes it, arrived at, whichever comes
 * first of the two.
 */
export function QuickSwitch({ notebook }: { notebook: Notebook }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const opener = useRef<HTMLElement | null>(null);
  const went = useRef(false);
  useEffect(() => {
    const mac = onMac();
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (!isMod(event, "o", mac)) {
        return;
      }
      event.preventDefault();
      if (document.querySelector('[role="dialog"], [role="alertdialog"]') === null) {
        opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        went.current = false;
        setOpen(true);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {open && (
        <DialogContent
          aria-describedby={undefined}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            if (!went.current && opener.current?.isConnected) {
              opener.current.focus();
            }
          }}
        >
          <DialogTitle>{t("page.quickSwitch")}</DialogTitle>
          <Finder
            notebook={notebook}
            leave={(going) => {
              went.current = going;
              setOpen(false);
            }}
          />
        </DialogContent>
      )}
    </Dialog>
  );
}

/**
 * Finder is the quick switch's field and the pages whose title holds what
 * is typed, each with its ancestors, the first 50: a combobox and its
 * listbox, Up and Down to move, Enter to go, and a status that says when
 * none is found or more are. Until the tree is read it says so, with Try
 * again.
 */
type FinderProps = {
  notebook: Notebook;
  /** leave closes the quick switch, going to another page or not. */
  leave: (going: boolean) => void;
};

const Finder = observer(function Finder({ notebook, leave }: FinderProps) {
  const pages = usePageTree(notebook);
  const { slug } = useWorkspace();
  const navigate = useNavigate();
  const { pageId } = useParams();
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
  // The tree read again may list fewer.
  const current = Math.min(active, Math.max(listed.length - 1, 0));

  function go(id: string) {
    // The page shown is no page to go to: the quick switch just closes.
    const going = id !== pageId;
    leave(going);
    if (going) {
      void navigate(`/${slug}/notebooks/${notebook.id}/pages/${id}`, { state: arrived });
    }
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive(Math.min(Math.max(current + step, 0), Math.max(listed.length - 1, 0)));
    } else if (event.key === "Enter") {
      event.preventDefault();
      const page = listed[current];
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
        aria-controls={listed.length > 0 ? ids.list : undefined}
        aria-autocomplete="list"
        aria-activedescendant={listed[current] === undefined ? undefined : optionId(current)}
        autoComplete="off"
        value={query}
        onChange={(event) => {
          setQuery(event.target.value);
          setActive(0);
        }}
        onKeyDown={onKeyDown}
      />
      {listed.length > 0 && (
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
              aria-selected={index === current}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => go(page.id)}
              className={cn("cursor-pointer px-3 py-1.5 text-sm", index === current && "bg-accent")}
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
      <output className="block text-xs text-muted-foreground empty:sr-only">
        {listed.length === 0
          ? t("page.quickSwitchNone")
          : found.length > shown
            ? t("page.quickSwitchFirst", { count: shown })
            : ""}
      </output>
    </div>
  );
});
