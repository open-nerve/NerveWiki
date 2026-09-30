import type { Translate } from "../i18n/i18n";
import { en, type MessageKey } from "../i18n/messages/en";
import { ApiError, type FieldError } from "../services/api";
import { SessionChangedError, SessionUnavailableError } from "../session/token-manager";

/**
 * The message of each problem code the pages show (M1/P5 design 3.4). A test
 * holds it to the contract: every code of the operations whose errors a page
 * shows has one.
 */
export const problemMessages = {
  bad_request: "problem.bad_request",
  payload_too_large: "problem.payload_too_large",
  rate_limited: "problem.rate_limited",
  internal_error: "problem.internal_error",
  unauthorized: "problem.unauthorized",
  validation_failed: "problem.validation_failed",
  server_busy: "problem.server_busy",
  "identity.signup_disabled": "problem.identity.signup_disabled",
  "identity.email_taken": "problem.identity.email_taken",
  "identity.invalid_credentials": "problem.identity.invalid_credentials",
  "identity.account_deactivated": "problem.identity.account_deactivated",
} as const satisfies Record<string, MessageKey>;

/** The message of each field code; `field.<field>.<code>` says it better for one field. */
const fieldMessages = {
  required: "field.required",
  invalid_format: "field.invalid_format",
  too_short: "field.too_short",
  too_long: "field.too_long",
  out_of_range: "field.out_of_range",
  not_allowed: "field.not_allowed",
  duplicate: "field.duplicate",
  common_password: "field.common_password",
} as const satisfies Record<FieldError["code"], MessageKey>;

/**
 * errorText is what a page says of error above its form or in place of what
 * it could not load, or undefined for a request cut by a change of session:
 * the page is going away. A 429 says how long to wait when the server said.
 */
export function errorText(error: unknown, t: Translate): string | undefined {
  if (error instanceof SessionChangedError) {
    return undefined;
  }
  if (error instanceof SessionUnavailableError) {
    return t("problem.unavailable");
  }
  if (error instanceof TypeError) {
    return t("problem.network");
  }
  if (error instanceof ApiError) {
    const code = error.code;
    if (code === "rate_limited" && error.retryAfter !== undefined) {
      return t("problem.rate_limitedFor", { seconds: error.retryAfter });
    }
    if (code !== undefined && Object.hasOwn(problemMessages, code)) {
      return t(problemMessages[code as keyof typeof problemMessages]);
    }
    return t("problem.other", { code: code ?? `HTTP ${error.status}` });
  }
  return t("problem.other", { code: error instanceof Error ? error.name : "?" });
}

/** fieldErrors is the message of each invalid field of a 422, by the field's name. */
export function fieldErrors(error: unknown, t: Translate): Record<string, string> {
  const errors = error instanceof ApiError ? (error.problem?.errors ?? []) : [];
  const byField: Record<string, string> = {};
  for (const { field, code } of errors) {
    const specific = `field.${field}.${code}` as const;
    byField[field] ??= t(isFieldMessage(specific) ? specific : fieldMessages[code]);
  }
  return byField;
}

/** The keys of the fields' messages, none of which has a placeholder. */
type FieldMessage = Extract<MessageKey, `field.${string}`>;

function isFieldMessage(key: `field.${string}`): key is FieldMessage {
  return Object.hasOwn(en, key);
}
