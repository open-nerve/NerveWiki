import { useEffect, useRef, useState } from "react";

import { errorText } from "../app/problem-messages";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";

/** StepProps are what a step's form takes: complete records the step, which shows the next. */
export type StepProps = { complete: () => Promise<void> };

/**
 * GoOn completes the step as it shows, once, where it has nothing to ask
 * (the workspace and notebook steps): its effect runs again with each new
 * complete, and twice in development, which the ref keeps to the first. A
 * failure says why, with a way to try again.
 */
export function GoOn({ complete }: StepProps) {
  const t = useT();
  const [failure, setFailure] = useState<unknown>();
  const started = useRef(false);

  function retry() {
    setFailure(undefined);
    complete().catch(setFailure);
  }

  useEffect(() => {
    if (!started.current) {
      started.current = true;
      complete().catch(setFailure);
    }
  }, [complete]);

  const failed = failure === undefined ? undefined : errorText(failure, t);
  if (failed === undefined) {
    return <output className="block text-muted-foreground">{t("onboarding.goingOn")}</output>;
  }
  return (
    <div className="space-y-3">
      <Alert>{failed}</Alert>
      <Button variant="outline" onClick={retry}>
        {t("status.retry")}
      </Button>
    </div>
  );
}
