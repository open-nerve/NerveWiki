import type { Middleware } from "@nervewiki/api-client";
import { SessionChangedError, type TokenManager } from "./token-manager";

/**
 * Puts the access token on every request of the client, and answers a 401 to it (M1/P5 design 3.2):
 * renews the token once and sends a copy of the request again, once. Two outcomes end the session:
 * the refresh answers 401 (the token manager ends it), or the request sent again is refused as well. A
 * refresh that fails for a passing reason (429, 5xx, no network) rejects the request with that error and
 * keeps the session. Without a session the request goes out without a token, and its 401 comes back as is.
 * A fetch that fails, the request's or its copy's, fails the request as it is and keeps the session.
 *
 * The client serves one session, loginId (undefined: none), the session of the stores it was built for: a
 * request made once the tab is in another session rejects with SessionChangedError before it is sent. A
 * request belongs to that session, and a copy refused again ends only that session. Whatever the answer, the
 * request rejects with SessionChangedError as well when the tab's state shows it has left the session by the
 * time the answer comes back, the request's own or its copy's, and when the renewal finds the change under
 * the lock: the server may have done what was asked, but its caller, of stores the tab has left, does not go
 * on to navigate or show it in the tab's new session. After the copy's 401, no state is checked:
 * endSession(loginId) decides, and its false (the record under the lock is no longer that session's, even
 * when the tab has not heard of the change yet) rejects the request with SessionChangedError. One signal for
 * a request cut by a change of session, never an answer its caller could take for the tab's session's.
 */
export function authMiddleware(
  tokens: Pick<TokenManager, "state" | "accessToken" | "renew" | "endSession">,
  loginId: string | undefined
): Middleware {
  // A copy of each request made before fetch reads its body, and the token it went with; openapi-fetch hands
  // onResponse the request object onRequest returned.
  const sent = new WeakMap<Request, { copy: Request; token: string }>();
  return {
    async onRequest({ request }) {
      // Checked right before accessToken() is called, so the token it gives is this session's.
      if (tokens.state.loginId !== loginId) throw new SessionChangedError();
      const token = await tokens.accessToken();
      if (token === undefined) return undefined;
      request.headers.set("Authorization", `Bearer ${token}`);
      sent.set(request, { copy: request.clone(), token });
      return request;
    },
    async onResponse({ request, response, options }) {
      // The tab is no longer in the request's session: the request stops, its answer unread, not renewed or
      // sent again.
      if (tokens.state.loginId !== loginId) throw new SessionChangedError();
      const first = sent.get(request);
      if (response.status !== 401 || first === undefined) return undefined;
      const token = await tokens.renew(first.token);
      if (token === undefined) return undefined;
      first.copy.headers.set("Authorization", `Bearer ${token}`);
      const again = await options.fetch(first.copy);
      if (again.status === 401) {
        // Refused again: the request's session ends, unless the record is no longer that session's by then.
        if (!(await tokens.endSession(loginId))) throw new SessionChangedError();
        return again;
      }
      if (tokens.state.loginId !== loginId) throw new SessionChangedError();
      return again;
    },
  };
}
