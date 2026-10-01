import { useId } from "react";

import { useT } from "../../i18n/i18n";
import type { MessageKey } from "../../i18n/messages/en";
import type { WorkspaceAccess } from "../../services/notebook.service";

/** The workspace accesses a notebook can have, from the closest (M3 design 4). */
const accesses = [
  { value: "none", label: "access.none", hint: "access.noneHint" },
  { value: "viewer", label: "access.viewer", hint: "access.viewerHint" },
  { value: "editor", label: "access.editor", hint: "access.editorHint" },
] as const satisfies readonly { value: WorkspaceAccess; label: MessageKey; hint: MessageKey }[];

type AccessOptionsProps = {
  value: WorkspaceAccess;
  onChange: (access: WorkspaceAccess) => void;
};

/**
 * AccessOptions are a notebook's workspace accesses as radio buttons, each
 * with what it opens to whom (M3/P4 design 3.3, 3.4): the creation
 * dialog's and the general page's. Choosing one only changes the form: the
 * arrow keys choose as they move.
 */
export function AccessOptions({ value, onChange }: AccessOptionsProps) {
  const t = useT();
  const id = useId();
  return (
    <fieldset className="space-y-3">
      <legend className="mb-2 text-sm font-medium">{t("access.legend")}</legend>
      {accesses.map((access) => (
        <div key={access.value} className="flex items-start gap-2">
          <input
            type="radio"
            id={`${id}-${access.value}`}
            name={id}
            value={access.value}
            checked={value === access.value}
            aria-describedby={`${id}-${access.value}-hint`}
            onChange={() => onChange(access.value)}
            className="mt-1"
          />
          <div>
            <label htmlFor={`${id}-${access.value}`} className="text-sm font-medium">
              {t(access.label)}
            </label>
            <p id={`${id}-${access.value}-hint`} className="text-sm text-muted-foreground">
              {t(access.hint)}
            </p>
          </div>
        </div>
      ))}
    </fieldset>
  );
}
