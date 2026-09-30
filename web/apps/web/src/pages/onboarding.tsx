import { observer } from "mobx-react-lite";
import { Suspense } from "react";
import { Navigate, useSearchParams } from "react-router";

import { safeNextPath } from "../app/next-path";
import { Loading } from "../components/loading";
import { useT } from "../i18n/i18n";
import { useAccount } from "../stores/context";
import { onboardingSteps, pendingSteps, type OnboardingStep } from "../onboarding/steps";

/** OnboardingPage takes the account through the steps it has left; once none is left, it goes to next. */
export function OnboardingPage() {
  return <Onboarding steps={onboardingSteps} />;
}

/** Onboarding shows the first of steps the account has not completed; completing it records it, which shows the next. */
export const Onboarding = observer(function Onboarding({ steps }: { steps: readonly OnboardingStep[] }) {
  const { account, me } = useAccount();
  const t = useT();
  const [params] = useSearchParams();
  const step = pendingSteps(me, steps)[0];
  if (step === undefined) {
    return <Navigate replace to={safeNextPath(params.get("next")) ?? "/"} />;
  }
  const { id, title, Component } = step;
  return (
    <section className="mx-auto max-w-sm space-y-6">
      <p className="text-sm text-muted-foreground">
        {t("onboarding.progress", { current: steps.indexOf(step) + 1, total: steps.length })}
      </p>
      <h1 className="text-2xl font-semibold">{t(title)}</h1>
      <Suspense fallback={<Loading />}>
        <Component key={id} complete={async () => void (await account.recordStep(id))} />
      </Suspense>
    </section>
  );
});
