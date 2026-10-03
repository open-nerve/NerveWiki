import type { PlainKey, Translate } from "../i18n/i18n";
import { en, type MessageKey } from "../i18n/messages/en";
import { ApiError, type FieldError } from "../services/api";
import { SessionChangedError, SessionStorageError, SessionUnavailableError } from "../session/token-manager";

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
  forbidden: "problem.forbidden",
  validation_failed: "problem.validation_failed",
  server_busy: "problem.server_busy",
  "identity.signup_disabled": "problem.identity.signup_disabled",
  "identity.email_taken": "problem.identity.email_taken",
  "identity.invalid_credentials": "problem.identity.invalid_credentials",
  "identity.account_deactivated": "problem.identity.account_deactivated",
  "identity.current_password_incorrect": "problem.identity.current_password_incorrect",
  "identity.api_token_not_found": "problem.identity.api_token_not_found",
  "workspace.not_found": "problem.workspace.not_found",
  "workspace.creation_disabled": "problem.workspace.creation_disabled",
  "workspace.slug_taken": "problem.workspace.slug_taken",
  "workspace.member_not_found": "problem.workspace.member_not_found",
  "workspace.own_membership": "problem.workspace.own_membership",
  "workspace.sole_admin": "problem.workspace.sole_admin",
  "workspace.invitation_not_found": "problem.workspace.invitation_not_found",
  "workspace.invitation_email_mismatch": "problem.workspace.invitation_email_mismatch",
  "workspace.no_admin": "problem.workspace.no_admin",
  "notebook.not_found": "problem.notebook.not_found",
  "notebook.member_not_found": "problem.notebook.member_not_found",
  "notebook.own_membership": "problem.notebook.own_membership",
  "notebook.sole_admin": "problem.notebook.sole_admin",
  "page.not_found": "problem.page.not_found",
  "page.cycle": "problem.page.cycle",
  "page.title_taken": "problem.page.title_taken",
  "page.too_deep": "problem.page.too_deep",
  "page.revision_mismatch": "problem.page.revision_mismatch",
  "page.edit_session_ended": "problem.page.edit_session_ended",
  "page.edit_session_not_found": "problem.page.edit_session_not_found",
  "page.edit_session_taken_over": "problem.page.edit_session_taken_over",
  "page.edit_session_unlocked": "problem.page.edit_session_unlocked",
  "page.locked": "problem.page.locked",
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
 * ProblemTexts are a page's own texts for problem codes that mean more
 * there than they say elsewhere, such as workspace.sole_admin to a
 * deactivation: the text of each code's key instead of its usual one.
 */
export type ProblemTexts = Readonly<Partial<Record<keyof typeof problemMessages, PlainKey>>>;

/**
 * errorText is what a page says of error above its form or in place of what
 * it could not load, or undefined for a request cut by a change of session:
 * the page is going away. A 429 says how long to wait when the server said.
 * texts says some codes the page's way.
 */
export function errorText(error: unknown, t: Translate, texts: ProblemTexts = {}): string | undefined {
  if (error instanceof SessionChangedError) {
    return undefined;
  }
  if (error instanceof SessionUnavailableError) {
    return t("problem.unavailable");
  }
  if (error instanceof SessionStorageError) {
    return t("problem.storage");
  }
  if (error instanceof TypeError) {
    return t("problem.network");
  }
  if (error instanceof ApiError) {
    const code = error.code;
    const own = code !== undefined && Object.hasOwn(texts, code) ? texts[code as keyof ProblemTexts] : undefined;
    if (own !== undefined) {
      return t(own);
    }
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

/**
 * FieldTexts are a form's own texts for some field codes of its fields,
 * keyed "<field>.<code>", where a field's code means more there than the
 * global text says (v0.1 design 13.2, item 11): a notebook's name is held
 * to the page title's rules, a workspace's to others.
 */
export type FieldTexts = Readonly<Partial<Record<`${string}.${FieldError["code"]}`, FieldMessage>>>;

/**
 * fieldErrors is the message of each invalid field of a 422, by the
 * field's name: the form's own text of the code, else the field's, else the
 * code's.
 */
export function fieldErrors(error: unknown, t: Translate, fieldTexts: FieldTexts = {}): Record<string, string> {
  const errors = error instanceof ApiError ? (error.problem?.errors ?? []) : [];
  const byField: Record<string, string> = {};
  for (const { field, code } of errors) {
    const specific = `field.${field}.${code}` as const;
    byField[field] ??= t(fieldTexts[`${field}.${code}`] ?? (isFieldMessage(specific) ? specific : fieldMessages[code]));
  }
  return byField;
}

/** FieldMessage is the key of a field's problem's text; none has a placeholder. */
export type FieldMessage = Extract<MessageKey, `field.${string}`>;

function isFieldMessage(key: `field.${string}`): key is FieldMessage {
  return Object.hasOwn(en, key);
}

/**
 * formErrors is what a form that shows the fields shown makes of error:
 * each field's problem under it, and the rest above the form. A 422 whose
 * problems are all on fields shown shows nothing above; a problem on a
 * field the form does not show goes above as the 422's text. A problem
 * code in onField shows under its field instead, such as a wrong current
 * password under the current password; texts says some codes the form's
 * way, fieldTexts some field codes. No error shows nothing.
 */
export function formErrors(
  error: unknown,
  t: Translate,
  shown: readonly string[],
  {
    onField = {},
    texts = {},
    fieldTexts = {},
  }: { onField?: Readonly<Record<string, string>>; texts?: ProblemTexts; fieldTexts?: FieldTexts } = {}
): { banner: string | undefined; fields: Record<string, string> } {
  if (error === undefined) {
    return { banner: undefined, fields: {} };
  }
  const codeField = error instanceof ApiError && error.code !== undefined ? onField[error.code] : undefined;
  if (codeField !== undefined) {
    return { banner: undefined, fields: { [codeField]: errorText(error, t, texts) ?? "" } };
  }
  const fields = fieldErrors(error, t, fieldTexts);
  const onFields = Object.keys(fields);
  const allShown =
    error instanceof ApiError &&
    error.code === "validation_failed" &&
    onFields.length > 0 &&
    onFields.every((field) => shown.includes(field));
  return { banner: allShown ? undefined : errorText(error, t, texts), fields };
}
