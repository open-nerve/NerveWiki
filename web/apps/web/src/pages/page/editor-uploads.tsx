import { observer } from "mobx-react-lite";

import type { Notebook } from "../../services/notebook.service";
import { useAssets } from "../../stores/context";
import { UploadRows } from "./upload-rows";

/**
 * EditorUploads are the uploads the page's editor began (M7/P4 design
 * 5.3), by the editor: their rows, which the attachments' section shows
 * once the page is read (its notices still say what begins and what is
 * uploaded), and what the editor last told (told: files uploaded and not
 * inserted, folders not uploaded), which the editor says unseen itself.
 * A row that leaves with the focus in it gives it to left.
 */
export const EditorUploads = observer(function EditorUploads({
  notebook,
  page,
  told,
  left,
}: {
  notebook: Notebook;
  page: string;
  told: string;
  left: () => void;
}) {
  const assets = useAssets(notebook);
  const uploads = assets.uploads.filter((upload) => upload.parent === page && upload.fromEditor);
  return (
    <div className="space-y-2 empty:hidden">
      {uploads.length > 0 && <UploadRows assets={assets} uploads={uploads} left={left} />}
      {told !== "" && <p className="text-sm whitespace-pre-line text-muted-foreground">{told}</p>}
    </div>
  );
});
