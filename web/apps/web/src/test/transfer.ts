import type { Transfer } from "../services/upload-fetch";
import type { Answer } from "./fakes";

/**
 * FakeTransfer is an XMLHttpRequest a test drives (M7/P4 design 3.2):
 * what was opened, the headers set and the body sent; on send it tells
 * sent, and the test answers it, fails it, or tells its progress. An
 * abort is answered as the browser does: the abort event, no answer.
 */
export class FakeTransfer extends EventTarget implements Transfer {
  readonly upload = new EventTarget();
  method = "";
  url = "";
  readonly headers = new Headers();
  body: Document | XMLHttpRequestBodyInit | null | undefined = undefined;
  status = 0;
  statusText = "";
  responseText = "";
  aborted = false;
  #responseHeaders = "";

  constructor(private readonly sent: (transfer: FakeTransfer) => void = () => undefined) {
    super();
  }

  open(method: string, url: string | URL): void {
    this.method = method;
    this.url = String(url);
  }

  setRequestHeader(name: string, value: string): void {
    this.headers.set(name, value);
  }

  send(body?: Document | XMLHttpRequestBodyInit | null): void {
    this.body = body;
    this.sent(this);
  }

  abort(): void {
    this.aborted = true;
    this.dispatchEvent(new Event("abort"));
  }

  getAllResponseHeaders(): string {
    return this.#responseHeaders;
  }

  /** progress tells that loaded bytes of total have gone. */
  progress(loaded: number, total: number): void {
    this.upload.dispatchEvent(new ProgressEvent("progress", { loaded, total, lengthComputable: true }));
  }

  /** answer answers with response, as the browser has it: its status, its text, its headers' lines. */
  async answer(response: Response): Promise<void> {
    this.responseText = await response.text();
    this.status = response.status;
    this.statusText = response.statusText;
    this.#responseHeaders = [...response.headers].map(([name, value]) => `${name}: ${value}\r\n`).join("");
    this.dispatchEvent(new Event("load"));
  }

  /** fail fails the request as the network would. */
  fail(): void {
    this.dispatchEvent(new Event("error"));
  }
}

/**
 * formOf is the form a transfer of transferTo sent with the request its
 * answer gets: the test's Request cannot carry the page's FormData.
 */
const forms = new WeakMap<Request, FormData>();

export function formOf(request: Request): FormData | undefined {
  return forms.get(request);
}

/** The transfer each request of transferTo's went by. */
const transfersOf = new WeakMap<Request, FakeTransfer>();

/** abortedFor tells whether the transfer of a request of transferTo's was aborted: what it sent did not all go. */
export function abortedFor(request: Request): boolean {
  return transfersOf.get(request)?.aborted === true;
}

/**
 * transferTo is the transfers of a test's app: each sends its request to
 * answer, as the page's other requests go, its form beside it (formOf);
 * as the answer comes, the whole body has gone: an answer held is a body
 * still going. The transfers made are in made.
 */
export function transferTo(answer: Answer): (() => FakeTransfer) & { made: FakeTransfer[] } {
  const made: FakeTransfer[] = [];
  const make = () => {
    const transfer = new FakeTransfer((sent) => {
      const request = new Request(sent.url, { method: sent.method, headers: sent.headers });
      const file = sent.body instanceof FormData ? sent.body.get("file") : null;
      const size = file instanceof Blob ? file.size : 0;
      if (sent.body instanceof FormData) {
        forms.set(request, sent.body);
      }
      transfersOf.set(request, sent);
      void (async () => {
        let response: Response;
        try {
          response = await answer(request);
        } catch {
          if (!sent.aborted) {
            sent.fail();
          }
          return;
        }
        if (!sent.aborted) {
          sent.progress(size, size);
          await sent.answer(response);
        }
      })();
    });
    made.push(transfer);
    return transfer;
  };
  return Object.assign(make, { made });
}
