import type { Page } from "@nervewiki/api-client";
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

/** A prompt as prompts/list tells it. */
interface McpPrompt {
  name: string;
  title?: string;
  description?: string;
  arguments?: { name: string; description?: string; required?: boolean }[];
}

/** A JSON-RPC error as the wire has it. */
interface McpRpcError {
  code: number;
  message: string;
  data?: { code?: string };
}

/** A prompt's answer: its description, and its messages' text. */
interface McpPromptResult {
  description?: string;
  messages: { role: string; content: { type: string; text: string } }[];
}

/** A scripted MCP client of one entry, initialized (M9 design 9). */
export interface McpClient {
  readonly protocolVersion: string;
  readonly instructions: string | undefined;
  listTools(): Promise<McpTool[]>;
  call(tool: string, args?: Record<string, unknown>): Promise<McpAnswer>;
  listPrompts(): Promise<McpPrompt[]>;
  /** A prompts/get, the one request whose JSON-RPC error is an answer: a standard notebook's refusal is LW11's. */
  getPrompt(name: string, args?: Record<string, string>): Promise<{ result: McpPromptResult } | { error: McpRpcError }>;
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
 * The answer of message, a request (with an id) or a notification (without), sent to url: a request answered 200
 * application/json, a notification 202 with an empty body, which has none. Anything else throws, with the status and
 * the answer.
 */
async function exchange(
  url: string,
  credential: string,
  version: string | undefined,
  message: Record<string, unknown>
): Promise<{ result?: Record<string, unknown>; error?: McpRpcError } | undefined> {
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
  return JSON.parse(body) as { result?: Record<string, unknown>; error?: McpRpcError };
}

/** The result of message, as exchange answers it: a JSON-RPC error, or no result of a request, throws. */
async function send(url: string, credential: string, version: string | undefined, message: Record<string, unknown>) {
  const answer = await exchange(url, credential, version, message);
  if (answer === undefined) {
    return undefined;
  }
  if (answer.error !== undefined || answer.result === undefined) {
    throw new Error(`the request ${String(message.method)} at ${url}: ${JSON.stringify(answer)}, want its result`);
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
    listPrompts: async () => ((await request("prompts/list", {}))?.prompts ?? []) as McpPrompt[],
    getPrompt: async (name, args = {}) => {
      const message = { jsonrpc: "2.0", id: ++id, method: "prompts/get", params: { name, arguments: args } };
      const answer = await exchange(url, credential, protocolVersion, message);
      if (answer?.error !== undefined) {
        return { error: answer.error };
      }
      if (answer?.result === undefined) {
        throw new Error(`prompts/get ${name} at ${url}: ${JSON.stringify(answer)}, want a result or an error`);
      }
      return { result: answer.result as unknown as McpPromptResult };
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

/**
 * The page a write or an edit wrote, as its answer tells it (M9/P5 design 3.1): its node, the revision its version
 * names, and the changeset's time, which is the content's; what expectContentWritten reads of a page REST answers. A
 * version not of the node, or no time (the call wrote nothing), fails rather than give NaN or "null"; what names the
 * call.
 */
export function writtenOf(
  answer: { node_id: string; version: string; at: string | null },
  what: string
): Pick<Page, "id" | "revision" | "content_updated_at"> {
  const revision = answer.version.startsWith(`${answer.node_id}:`)
    ? answer.version.slice(answer.node_id.length + 1)
    : "";
  if (!/^[1-9][0-9]*$/.test(revision) || answer.at === null) {
    throw new Error(`${what} answered ${JSON.stringify(answer)}: want a version of its node and the time it wrote`);
  }
  return { id: answer.node_id, revision: Number(revision), content_updated_at: answer.at };
}

interface PageReadMeta {
  path: string;
  kind: "page";
  offset: number;
  column: number;
  lines: number;
  total_lines: number;
  byte_size: number;
  next_offset: number | null;
  next_column: number | null;
}

/** A generated page has no version. Real reads retain their required version. */
export interface PageRead {
  meta: PageReadMeta & ({ virtual: true; version?: never } | { virtual?: false; version: string });
  text: string;
}

/** Narrow before a story uses a real page's version as a write precondition. */
export function assertRealPage(read: PageRead): asserts read is PageRead & {
  meta: PageReadMeta & { virtual?: false; version: string };
} {
  if (read.meta.virtual === true || typeof read.meta.version !== "string" || read.meta.version.length === 0) {
    throw new Error("Expected a real page read with a nonempty version");
  }
}

function pageReadMeta(value: unknown): PageRead["meta"] {
  if (typeof value !== "object" || value === null) throw new Error("Invalid page read metadata");
  const fields = new Map(Object.entries(value));
  const integer = (name: string, minimum: number): number => {
    const n: unknown = fields.get(name);
    if (typeof n !== "number" || !Number.isSafeInteger(n) || n < minimum) {
      throw new Error(`Invalid page read ${name}`);
    }
    return n;
  };
  const nullableInteger = (name: string, minimum: number) =>
    fields.get(name) === null ? null : integer(name, minimum);
  const path: unknown = fields.get("path");
  if (typeof path !== "string" || path.length === 0 || fields.get("kind") !== "page") {
    throw new Error("Invalid page read path or kind");
  }
  const base: PageReadMeta = {
    path,
    kind: "page",
    offset: integer("offset", 1),
    column: integer("column", 0),
    lines: integer("lines", 0),
    total_lines: integer("total_lines", 0),
    byte_size: integer("byte_size", 0),
    next_offset: nullableInteger("next_offset", 1),
    next_column: nullableInteger("next_column", 0),
  };
  const virtual: unknown = fields.get("virtual");
  if (virtual === true) {
    if (fields.has("version")) throw new Error("Virtual read exposed a version");
    return { ...base, virtual: true };
  }
  const version: unknown = fields.get("version");
  if ((virtual !== undefined && virtual !== false) || typeof version !== "string" || version.length === 0) {
    throw new Error("Real page read has no valid version");
  }
  return { ...base, virtual: false, version };
}

/** read's answer of a page's path through agent, from offset and column: two text blocks, no structuredContent. */
export async function readPage(agent: McpClient, args: Record<string, unknown>): Promise<PageRead> {
  const answer = await agent.call("read", args);
  expect(answer.isError, `read ${JSON.stringify(args)}: ${answer.text.join(" ")}`).toBe(false);
  expect(answer.structured, "a text tool's answer has no structuredContent").toBeUndefined();
  expect(answer.content.map((c) => c.type)).toEqual(["text", "text"]);
  const meta: unknown = JSON.parse(answer.text[0] ?? "");
  return { meta: pageReadMeta(meta), text: answer.text[1] ?? "" };
}

/**
 * The line of path at line as read gives it, its line end included (M9/P4 design 4): read's second block, at offset
 * line, limit 1. The stories check grep's line numbers with it.
 */
export async function lineAt(agent: McpClient, path: string, line: number): Promise<string> {
  const answer = await agent.call("read", { path, offset: line, limit: 1 });
  expect(answer.isError, `read ${path} at ${line}: ${answer.text.join(" ")}`).toBe(false);
  return answer.text[1] ?? "";
}
