import { PanelRight } from "lucide-react";
import { observer } from "mobx-react-lite";
import type { Ref } from "react";

import { Button } from "../../components/ui/button";
import { formatDateTime, formatRelative } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { isolate, Isolated } from "../../i18n/isolated";
import type { Notebook } from "../../services/notebook.service";
import { useStore } from "../../stores/context";
import { useVersions } from "./page-versions";

// The page's head in one row (M13/P4/S1, 4.1.3): what its right group shows besides the page's buttons.

/**
 * HeadMeta is who edited the page last, and when (M13/P4/S1, decision 858): its newest version's, read by the key the
 * right column reads the versions by (SWR reads them once for both), the name in a bdi, the time in full as its title;
 * empty until they are read, and for a page without versions; shortened with an ellipsis where it is long. A
 * paragraph, no status: not heard each time it comes, the edit's status the main's only one. As long ago as it was
 * when it was read: it does not go on counting (M5-polish's known limit).
 */
export const HeadMeta = observer(function HeadMeta({ notebook, page }: { notebook: Notebook; page: string }) {
  const t = useT();
  const { preferences } = useStore();
  const { data } = useVersions(notebook, page);
  const newest = data?.[0]?.data[0];
  const { locale } = preferences;
  return (
    <p title={newest === undefined ? undefined : formatDateTime(newest.updated_at, locale)} className="truncate">
      {newest !== undefined && (
        <Isolated
          {...isolate((name) =>
            t("page.editedBy", {
              name: name(newest.changeset.created_by.display_name),
              when: formatRelative(newest.updated_at, Date.now(), locale),
            })
          )}
        />
      )}
    </p>
  );
});

/**
 * PanelToggle folds the page's right column away and shows it again (M13/P4/S1, decision 859), for this browser's
 * next pages too: named as the column is, expanded as it is open, controlling it (controls, its id). Only where the
 * column is beside the page (xl up); narrower the column is after the page, always shown. The focus stays on it.
 */
export const PanelToggle = observer(function PanelToggle({
  ref,
  controls,
}: {
  ref: Ref<HTMLButtonElement>;
  controls: string;
}) {
  const t = useT();
  const { preferences } = useStore();
  return (
    <Button
      ref={ref}
      variant="ghost"
      size="icon-sm"
      shape="pill"
      className="hidden xl:inline-flex"
      aria-label={t("page.panel")}
      aria-expanded={preferences.panelOpen}
      aria-controls={controls}
      onClick={() => preferences.setPanelOpen(!preferences.panelOpen)}
    >
      <PanelRight />
    </Button>
  );
});
