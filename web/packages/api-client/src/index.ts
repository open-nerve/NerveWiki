import createFetchClient, { type ClientOptions } from "openapi-fetch";

// What the web app's session puts on its clients (client.use).
export type { Middleware } from "openapi-fetch";

import type { paths } from "./schema.gen";

// The schemas under their own names (InstanceInfo, Problem …): openapi-typescript's --root-types.
export type * from "./schema.gen";

/**
 * Creates a client for the Nerve Wiki API. Paths, parameters, request bodies
 * and responses are typed from api/dist/openapi.yaml; error bodies are
 * problem+json, Problem.
 */
export function createClient(options?: ClientOptions) {
  return createFetchClient<paths>(options);
}

/** A client of the Nerve Wiki API, as createClient makes it. */
export type ApiClient = ReturnType<typeof createClient>;
