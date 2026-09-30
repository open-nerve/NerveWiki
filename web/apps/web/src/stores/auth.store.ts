import type { AuthService } from "../services/auth.service";
import type { SessionState, TokenManager } from "../session/token-manager";

export type { SessionState };

/**
 * AuthStore is the tab's session as the pages see it: its state, and
 * signing in, up and out. Signing in or up keeps the new session's tokens,
 * which starts a new generation of the stores; where the page goes next is
 * the route guards' to decide (M1/P5 design 3.5).
 */
export class AuthStore {
  constructor(
    private readonly service: Pick<AuthService, "login" | "register">,
    private readonly tokens: Pick<TokenManager, "state" | "subscribe" | "retry" | "signIn" | "signOut" | "endSession">,
    /** The login of this generation of the stores; undefined while signed out. */
    private readonly loginId: string | undefined
  ) {}

  /** The session's state: a new object on every change (for useSyncExternalStore). */
  get state(): SessionState {
    return this.tokens.state;
  }

  /** Calls listener after every change of state; returns the unsubscribe. */
  subscribe = (listener: () => void): (() => void) => this.tokens.subscribe(listener);

  /** retry tries the session's first refresh again now, as "Try again" asks. */
  retry = (): Promise<void> => this.tokens.retry();

  async signIn(email: string, password: string): Promise<void> {
    await this.tokens.signIn(await this.service.login(email, password));
  }

  async signUp(email: string, password: string): Promise<void> {
    await this.tokens.signIn(await this.service.register(email, password));
  }

  /** signOut ends the session in every tab of the browser. */
  signOut(): Promise<void> {
    return this.tokens.signOut();
  }

  /**
   * endSession forgets, in every tab of the browser, the session of this
   * generation that the server ended already (a deactivated account): no
   * logout goes out. A record that is another session's by now stays.
   */
  async endSession(): Promise<void> {
    await this.tokens.endSession(this.loginId);
  }
}
