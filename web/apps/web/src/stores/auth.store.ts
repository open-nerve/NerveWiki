import type { AuthService } from "../services/auth.service";
import type { TokenManager } from "../session/token-manager";

/**
 * AuthStore signs the tab in, up and out. Signing in or up keeps the new
 * session's tokens, which starts a new generation of the stores; where the
 * page goes next is the route guards' to decide (M1/P5 design 3.6).
 */
export class AuthStore {
  constructor(
    private readonly service: Pick<AuthService, "login" | "register">,
    private readonly tokens: Pick<TokenManager, "signIn" | "signOut">
  ) {}

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
}
