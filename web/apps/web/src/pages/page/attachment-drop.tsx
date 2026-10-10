import { useState, type ReactNode } from "react";

import { useFileDrop } from "../../app/file-drop";
import { Alert } from "../../components/ui/alert";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import type { Notebook } from "../../services/notebook.service";
import { useAssets, useStore } from "../../stores/context";

/**
 * AttachmentDrop is where files dropped upload under parent, as on its
 * attachments' section: the reading view of a page, for a writer (M7/P4
 * design 3.6). A folder is not uploaded: it says to import one.
 */
export function AttachmentDrop({
  notebook,
  parent,
  enabled,
  children,
}: {
  notebook: Notebook;
  parent: string | null;
  enabled: boolean;
  children: ReactNode;
}) {
  const t = useT();
  const assets = useAssets(notebook);
  const { instance } = useStore();
  const [folders, setFolders] = useState(false);
  // The section of parent says what began.
  const drop = useFileDrop(enabled, (dropped) => {
    setFolders(dropped.folders);
    assets.upload(parent, dropped.files, t("asset.untitled"), { maxBytes: instance.info?.asset_max_bytes });
  });
  return (
    <div className={cn("space-y-4 rounded-md", drop.over && "ring-2 ring-primary ring-offset-4")} {...drop.handlers}>
      {folders && <Alert>{t("asset.folders")}</Alert>}
      {children}
    </div>
  );
}
