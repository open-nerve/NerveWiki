import { expect } from "@playwright/test";

import { bearer } from "./auth";

// The scripted MCP client of the stories (M9 design 9; M9/P1 design 4.2):
// raw JSON-RPC through Node's fetch, one message a POST, as the server's
// stateless JSON answers have it.

/** The protocol version the client asks for. */
const askedVersion = "2025-11-25";

// McpContent, McpTool and McpAnswer are not exported until a story imports them: MC1 compares listTools' answer
// with api/mcp/tools.json without naming its type. knip counts a type used by an exported function's signature, not
// by an exported interface's members.

/** A content block of a tool's answer, as the wire has it. */
type McpContent =
  | { type: "text"; text: string }
  | { type: "image"; data: string; mimeType: string }
  | { type: "resource_link"; uri: string; name: string; mimeType?: string; size?: number };

/** A tool as tools/list tells it. */
interface McpTool {
  name: string;
  title?: string;
  description?: string;
  inputSchema: Record<string, unknown>;
  annotations?: Record<string, unknown>;
}

/** A tool's answer as a client hands it on: whether it is an error, its structuredContent, and its text blocks. */
interface McpAnswer {
  isError: boolean;
  structured: unknown;
  text: string[];
  content: McpContent[];
}

/** A scripted MCP client of one entry, initialized (M9 design 9). */
export interface McpClient {
  readonly protocolVersion: string;
  readonly instructions: string | undefined;
  listTools(): Promise<McpTool[]>;
  call(tool: string, args?: Record<string, unknown>): Promise<McpAnswer>;
}

/** The MCP address of baseURL: the account entry, or a notebook's. */
export function mcpURL(baseURL: string, notebook?: string): string {
  const url = new URL("/mcp", baseURL);
  if (notebook !== undefined) {
    url.searchParams.set("notebook", notebook);
  }
  return url.toString();
}

/** The headers of a POST to an entry: credential's, and the version negotiated once there is one. */
function headersOf(credential: string | undefined, version?: string): Record<string, string> {
  return {
    Accept: "application/json, text/event-stream",
    "Content-Type": "application/json",
    ...(credential === undefined ? {} : bearer(credential)),
    ...(version === undefined ? {} : { "MCP-Protocol-Version": version }),
  };
}

/** credential's POST of body to the entry, as the server answers it: for the refusals before the SDK. */
export function postMcp(
  baseURL: string,
  credential: string | undefined,
  body: unknown,
  notebook?: string
): Promise<Response> {
  return fetch(mcpURL(baseURL, notebook), {
    method: "POST",
    headers: headersOf(credential),
    body: JSON.stringify(body),
  });
}

/**
 * The result of message, a request (with an id) or a notification (without), sent to url: a request answered 200
 * application/json without a JSON-RPC error, a notification 202 with an empty body. Anything else throws, with the
 * status and the answer.
 */
async function send(url: string, credential: string, version: string | undefined, message: Record<string, unknown>) {
  const response = await fetch(url, {
    method: "POST",
    headers: headersOf(credential, version),
    body: JSON.stringify(message),
  });
  const body = await response.text();
  const what = `${String(message.method)} at ${url}: ${response.status} ${body}`;
  if (!("id" in message)) {
    if (response.status !== 202 || body !== "") {
      throw new Error(`the notification ${what}, want 202 and no body`);
    }
    return undefined;
  }
  if (response.status !== 200 || !(response.headers.get("content-type") ?? "").startsWith("application/json")) {
    throw new Error(`the request ${what}, want 200 application/json`);
  }
  const answer = JSON.parse(body) as { result?: Record<string, unknown>; error?: unknown };
  if (answer.error !== undefined || answer.result === undefined) {
    throw new Error(`the request ${what}, want its result`);
  }
  return answer.result;
}

/**
 * credential's client of the entry, initialized as client (default "e2e-agent"), then notifications/initialized sent.
 * With handshake false it sends neither, its requests carrying the version it asks for, as a client's whose server
 * restarted (M9/P2 design 4.1): the name its token recorded is then its calls' client.
 */
export async function mcpClient(
  baseURL: string,
  credential: string,
  options: { notebook?: string; client?: string; handshake?: boolean } = {}
): Promise<McpClient> {
  const url = mcpURL(baseURL, options.notebook);
  let id = 0;
  let protocolVersion = askedVersion;
  let instructions: string | undefined;
  if (options.handshake !== false) {
    const init = await send(url, credential, undefined, {
      jsonrpc: "2.0",
      id: ++id,
      method: "initialize",
      params: {
        protocolVersion: askedVersion,
        capabilities: {},
        clientInfo: { name: options.client ?? "e2e-agent", version: "1" },
      },
    });
    protocolVersion = String(init?.protocolVersion);
    instructions = typeof init?.instructions === "string" ? init.instructions : undefined;
    await send(url, credential, protocolVersion, { jsonrpc: "2.0", method: "notifications/initialized" });
  }
  const request = (method: string, params: Record<string, unknown>) =>
    send(url, credential, protocolVersion, { jsonrpc: "2.0", id: ++id, method, params });
  return {
    protocolVersion,
    instructions,
    listTools: async () => ((await request("tools/list", {}))?.tools ?? []) as McpTool[],
    call: async (tool, args = {}) => {
      const result = await request("tools/call", { name: tool, arguments: args });
      const content = (result?.content ?? []) as McpContent[];
      return {
        isError: result?.isError === true,
        structured: result?.structuredContent,
        text: content.flatMap((c) => (c.type === "text" ? [c.text] : [])),
        content,
      };
    },
  };
}

/** A tool's error as its structuredContent.error has it (M9 design 4.7): its code, its text, and its tool's members. */
export interface McpError {
  code: string;
  message: string;
  [member: string]: unknown;
}

/** answer's structuredContent, which must be no error; what names the call in a failure. */
export function resultOf<T>(answer: McpAnswer, what: string): T {
  expect(answer.isError, `${what}: ${answer.text.join(" ")}`).toBe(false);
  return answer.structured as T;
}

/** answer's error, which must be one, its text the error's message; what names the call in a failure. */
export function errorOf(answer: McpAnswer, what: string): McpError {
  expect(answer.isError, `${what}: ${JSON.stringify(answer.structured)}`).toBe(true);
  const { error } = answer.structured as { error: McpError };
  expect(answer.text, `${what}: the text is the error's message`).toEqual([error.message]);
  return error;
}
