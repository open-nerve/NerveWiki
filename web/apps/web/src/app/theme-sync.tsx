import { observer } from "mobx-react-lite";
import { useEffect } from "react";

import { useStore } from "../stores/context";

/** ThemeSync keeps the dark class on <html> in step with the resolved theme. */
export const ThemeSync = observer(function ThemeSync() {
  const theme = useStore().preferences.resolvedTheme;
  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark");
  }, [theme]);
  return null;
});
