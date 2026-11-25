import { useId, useState } from "react";

import { filterBarClass, filterFieldClass } from "../../app/page-classes";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Label } from "../../components/ui/label";
import { NativeSelect, useChoice } from "../../components/ui/native-select";
import { useT } from "../../i18n/i18n";
import type { IndexFilter, NotebookIndex } from "../../services/linking.service";
import { useStore } from "../../stores/context";

/** Only labels are shortened. Values, including a selected URL value outside the catalogue, stay exact. */
function label(value: string): string {
  const characters = Array.from(value);
  return characters.length > 80 ? characters.slice(0, 79).join("") + "…" : value;
}

export function IndexFilters({
  filter,
  catalogue,
  choose,
}: {
  filter: IndexFilter;
  catalogue: NotebookIndex["types"];
  choose: (filter: IndexFilter) => void;
}) {
  const t = useT();
  const { locale } = useStore().preferences;
  const typeId = useId();
  const tagId = useId();
  // Stepped through with the keys, a type reads the index once the reader leaves it or presses Enter (M5-polish 26.5).
  const type = useChoice(filter.type, (value) => choose({ ...filter, type: value }));
  const [tag, setTag] = useState(filter.tag);
  const [shown, setShown] = useState(filter.tag);
  if (shown !== filter.tag) {
    setShown(filter.tag);
    setTag(filter.tag);
  }
  const clear = filter.type !== "" || filter.tag !== "" || tag !== "";
  const outside = filter.type !== "" && !catalogue.some((option) => option.type === filter.type);
  return (
    <form
      className={filterBarClass}
      onSubmit={(event) => {
        event.preventDefault();
        choose({ ...filter, tag });
      }}
    >
      <div className={filterFieldClass}>
        <Label htmlFor={typeId}>{t("index.type")}</Label>
        {/* At most 16rem, as the history's Who: a long type widens it, and the bar, no more (P5S2B-N2). */}
        <NativeSelect id={typeId} {...type} className="max-w-64">
          <option value="">{t("index.allTypes")}</option>
          {outside && <option value={filter.type}>{label(filter.type)}</option>}
          {catalogue.map((option) => (
            <option key={option.type} value={option.type}>
              {t("index.typeCount", { type: label(option.type), count: option.count.toLocaleString(locale) })}
            </option>
          ))}
        </NativeSelect>
      </div>
      <div className={filterFieldClass}>
        <Label htmlFor={tagId}>{t("index.tag")}</Label>
        <Input id={tagId} value={tag} onChange={(event) => setTag(event.target.value)} />
      </div>
      <Button type="submit" variant="outline">
        {t("index.applyTag")}
      </Button>
      <Button
        type="button"
        variant="ghost"
        aria-disabled={!clear || undefined}
        className="aria-disabled:opacity-50"
        onClick={() => {
          setTag("");
          // Text not applied is the box's alone: a new address would read the list again and fold what More read.
          if (filter.type !== "" || filter.tag !== "") choose({ type: "", tag: "" });
        }}
      >
        {t("index.clearFilters")}
      </Button>
    </form>
  );
}
