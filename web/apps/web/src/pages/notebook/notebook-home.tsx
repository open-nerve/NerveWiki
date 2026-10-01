import { observer } from "mobx-react-lite";

import { useArrivalFocus } from "../../app/arrival";
import { useT } from "../../i18n/i18n";
import { useNotebook } from "./notebook-layout";

/** NotebookHomePage is a notebook's first page: M4 lists its pages here. */
export const NotebookHomePage = observer(function NotebookHomePage() {
  const notebook = useNotebook();
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  return (
    <section className="space-y-2">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
        {notebook.name}
      </h1>
      <p className="text-muted-foreground">{t("notebook.homeEmpty")}</p>
    </section>
  );
});
