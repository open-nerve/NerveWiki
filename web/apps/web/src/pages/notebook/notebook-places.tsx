import { HeartPulse, History, List, Trash2, type LucideIcon } from "lucide-react";
import { NavLink } from "react-router";

import { useFollow } from "../../app/shell-sidebar";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import type { Notebook } from "../../services/notebook.service";
import { useWorkspace } from "../workspace/workspace-layout";

/**
 * The places, in their order, each with its icon: an LLM Wiki's tree has Directory and History rows of the same
 * icons (decision 796).
 */
const places: readonly {
  path: string;
  label: "index.title" | "health.title" | "history.title" | "trash.title";
  icon: LucideIcon;
}[] = [
  { path: "index", label: "index.title", icon: List },
  { path: "health", label: "health.title", icon: HeartPulse },
  { path: "history", label: "history.title", icon: History },
  { path: "trash", label: "trash.title", icon: Trash2 },
];

/**
 * NotebookPlaces is the notebook's places other than its pages in the
 * left column, below its page tree (M8 design 4.13; M8/P2 design 4.5,
 * M8/P3 design 4.5, M10/P4): directory, health, history, then trash. A navigation of its
 * own, named: the tree's links are its pages, each, as the tree's tests
 * and readers take them. Each is marked current on its own page, not on
 * what is under it, a changeset's page or a trash item's preview, which
 * their breadcrumbs place under it (M8/P2B review A-N2). A row of icons
 * (M13/P2/S2, 00-M13 decision 727), each named by its text, which only a
 * screen reader reads, and titled by it for the pointer; the current one
 * as the tree's page shown is (decision 796). Followed as the tree's rows
 * are, so that a place gives the focus to the same from its row and its
 * icon (useFollow; decision 808).
 */
export function NotebookPlaces({ notebook }: { notebook: Notebook }) {
  const t = useT();
  const { slug } = useWorkspace();
  const home = `/${slug}/notebooks/${notebook.id}`;
  const follow = useFollow();
  return (
    <nav aria-label={t("notebook.places", { notebook: notebook.name })} className="flex gap-0.5 py-1 pl-6">
      {places.map(({ path, label, icon: Icon }) => (
        <NavLink
          key={path}
          end
          to={`${home}/${path}`}
          onClick={follow(`${home}/${path}`)}
          title={t(label)}
          className={({ isActive }) =>
            cn(
              "grid size-7 place-items-center rounded-row transition-colors duration-150 [&_svg]:size-4",
              isActive ? "bg-accent-soft [&_svg]:text-accent" : "text-faint hover:bg-hover hover:text-foreground"
            )
          }
        >
          <Icon aria-hidden />
          <span className="sr-only">{t(label)}</span>
        </NavLink>
      ))}
    </nav>
  );
}
