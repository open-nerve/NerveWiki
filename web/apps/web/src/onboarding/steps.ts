import { lazy, type ComponentType } from "react";

import type { MessageKey } from "../i18n/messages/en";
import type { User } from "../services/account.service";

/**
 * OnboardingStep is a step of onboarding (M1/P5 design 3.7). The server
 * keeps the ids of the steps an account has completed; which steps there
 * are is the app's to say, here.
 */
export type OnboardingStep = {
  /** The id the server records: a lower-case letter, then lower-case letters, digits and underscores, 32 at most. */
  id: string;
  title: Extract<MessageKey, `onboarding.${string}.title`>;
  /**
   * The step's form, loaded with the onboarding page (React.lazy): the guards read the registry on every
   * page, and need only the ids. It calls complete once its work is done, and shows why if complete fails.
   */
  Component: ComponentType<{ complete: () => Promise<void> }>;
};

/**
 * onboardingSteps are the steps, in the order they are taken. M2 and M3 add
 * theirs at the end: an account that completed the ones before sees only
 * the new one, on its next visit. The server records 32 steps at most.
 */
export const onboardingSteps: readonly OnboardingStep[] = [
  {
    id: "profile",
    title: "onboarding.profile.title",
    Component: lazy(async () => {
      const { ProfileStep } = await import("./profile-step");
      return { default: ProfileStep };
    }),
  },
];

/**
 * pendingSteps are the steps of steps that me has not completed, in their
 * order; none left is onboarding done. An id the server keeps that no step
 * has any more changes nothing.
 */
export function pendingSteps(
  me: Pick<User, "onboarding_steps">,
  steps: readonly OnboardingStep[] = onboardingSteps
): OnboardingStep[] {
  const done = new Set(me.onboarding_steps);
  return steps.filter((step) => !done.has(step.id));
}
