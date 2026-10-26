import { useId } from "react";

import { useT } from "../../i18n/i18n";
import type { MessageKey } from "../../i18n/messages/en";
import type { NotebookMode } from "../../services/notebook.service";

/** The modes offered at creation, the ordinary notebook first (M10/P1 design 4.4). */
const modes = [
  { value: "standard", label: "notebooks.modeStandard", hint: "notebooks.modeStandardHint" },
  { value: "llm_wiki", label: "notebooks.modeLLMWiki", hint: "notebooks.modeLLMWikiHint" },
] as const satisfies readonly { value: NotebookMode; label: MessageKey; hint: MessageKey }[];

type ModeOptionsProps = {
  value: NotebookMode;
  onChange: (mode: NotebookMode) => void;
};

/** ModeOptions chooses how the notebook starts; each mode describes how its pages are kept. */
export function ModeOptions({ value, onChange }: ModeOptionsProps) {
  const t = useT();
  const id = useId();
  return (
    <fieldset className="space-y-3">
      <legend className="mb-2 text-sm font-medium">{t("notebooks.modeLegend")}</legend>
      {modes.map((mode) => (
        <div key={mode.value} className="flex items-start gap-2">
          <input
            type="radio"
            id={`${id}-${mode.value}`}
            name={id}
            value={mode.value}
            checked={value === mode.value}
            aria-describedby={`${id}-${mode.value}-hint`}
            onChange={() => onChange(mode.value)}
            className="mt-1"
          />
          <div>
            <label htmlFor={`${id}-${mode.value}`} className="text-sm font-medium">
              {t(mode.label)}
            </label>
            <p id={`${id}-${mode.value}-hint`} className="text-sm text-muted-foreground">
              {t(mode.hint)}
            </p>
          </div>
        </div>
      ))}
    </fieldset>
  );
}
