/**
 * Transfer is the XMLHttpRequest an upload goes out on (M7/P4 design 3.2):
 * the browser's, or a test's.
 */
export type Transfer = Pick<
  XMLHttpRequest,
  | "open"
  | "setRequestHeader"
  | "send"
  | "abort"
  | "getAllResponseHeaders"
  | "status"
  | "statusText"
  | "responseText"
  | "addEventListener"
> & { readonly upload: Pick<XMLHttpRequestUpload, "addEventListener"> };

/** UploadOptions are how an upload tells its progress and is stopped. */
export type UploadOptions = {
  /** progress is called as the request's body goes out: the bytes sent of the bytes to send. */
  progress?: (sent: number, total: number) => void;
  /** signal stops the upload: its request rejects with an AbortError. */
  signal?: AbortSignal;
};

/** Statuses whose answer has no body: a Response with one throws. */
const bodiless = new Set([204, 205, 304]);

/**
 * stallLimit is how long a body going out may make no progress before the
 * upload is given up: bytes that flow tell their progress every few
 * seconds, at the server's least rate too; a connection cut without a word
 * or a server stuck tell nothing (M12/P2/S5, 16.3).
 */
const stallLimit = 60_000;

/**
 * answerLimit is how long a body all sent waits for its answer before the
 * upload is given up: the bytes still in the send buffer drain first, then
 * the server writes the file down and answers within its 15 seconds. It is
 * longer than stallLimit, for to give up aborts the request, which rolls
 * the server's transaction back: a wrong guess loses a sound upload
 * (M12/P2/S5, 16.3).
 */
const answerLimit = 180_000;

/**
 * UploadTimeout is why an upload was given up (M12/P2/S5, 16.3): its body
 * stalled, going nowhere for stallLimit; or the server, having it all,
 * left it unanswered for answerLimit. It is not an AbortError: the upload
 * was not cancelled.
 */
export class UploadTimeout extends Error {
  constructor(readonly stage: "stalled" | "unanswered") {
    super(stage === "stalled" ? "the upload stalled" : "the upload was not answered");
    this.name = "UploadTimeout";
  }
}

/**
 * uploadFetch is a fetch, as openapi-fetch calls it, that sends form on a
 * Transfer, which tells the upload's progress, as fetch does not (M7/P4
 * design 3.2). It takes the request's method, address and headers, the
 * token the session's middleware put there among them, not its body nor
 * its Content-Type: the transfer sends form, and writes the form's
 * Content-Type, its boundary in it. The middleware's copy of the request,
 * sent again after a renewed token, sends form again, whole. It answers
 * as fetch does: the answer's status, headers and text; a TypeError for a
 * network failure; an AbortError once signal aborts. It has no timeout of
 * the whole, which would cut a slow upload of a large file, but two of its
 * own: a body going nowhere for stallLimit, and one all sent unanswered for
 * answerLimit, is given up, rejecting with an UploadTimeout (M12/P2/S5,
 * 16.3). The body is all sent as the upload's load comes, or its progress
 * reaches its total: Firefox holds the load back until the answer comes.
 */
export function uploadFetch(
  form: FormData,
  { progress, signal }: UploadOptions = {},
  transfer: () => Transfer = () => new XMLHttpRequest()
): (request: Request) => Promise<Response> {
  return (request) =>
    new Promise<Response>((resolve, reject) => {
      if (signal?.aborted) {
        reject(aborted());
        return;
      }
      const xhr = transfer();
      const stop = () => xhr.abort();
      let timer: ReturnType<typeof setTimeout> | undefined;
      let timedOut: UploadTimeout | undefined;
      let settled = false;
      // One timer: each progress sets it again, and a body all sent waits for its answer instead; none once settled.
      const wait = (stage: UploadTimeout["stage"]) => {
        clearTimeout(timer);
        if (settled) {
          return;
        }
        timer = setTimeout(
          () => {
            timedOut = new UploadTimeout(stage);
            xhr.abort();
          },
          stage === "stalled" ? stallLimit : answerLimit
        );
      };
      const settle = () => {
        settled = true;
        clearTimeout(timer);
        signal?.removeEventListener("abort", stop);
      };
      xhr.open(request.method, request.url);
      request.headers.forEach((value, name) => {
        if (name.toLowerCase() !== "content-type") {
          xhr.setRequestHeader(name, value);
        }
      });
      xhr.upload.addEventListener("progress", (event) => {
        wait(event.lengthComputable && event.loaded >= event.total ? "unanswered" : "stalled");
        progress?.(event.loaded, event.total);
      });
      xhr.upload.addEventListener("load", () => wait("unanswered"));
      xhr.addEventListener("load", () => {
        settle();
        const body = bodiless.has(xhr.status) ? null : xhr.responseText;
        try {
          resolve(new Response(body, { status: xhr.status, statusText: xhr.statusText, headers: headersOf(xhr) }));
        } catch {
          // A status out of 200–599, or a header's name no Headers takes: no answer fetch would give.
          reject(new TypeError("the upload failed: an answer fetch does not take"));
        }
      });
      const failed = () => {
        settle();
        reject(new TypeError("the upload failed: network error"));
      };
      xhr.addEventListener("error", failed);
      xhr.addEventListener("abort", () => {
        settle();
        reject(timedOut ?? aborted());
      });
      signal?.addEventListener("abort", stop, { once: true });
      xhr.send(form);
      wait("stalled");
    });
}

/** aborted is the error a stopped upload rejects with, fetch's. */
function aborted(): DOMException {
  return new DOMException("the upload was stopped", "AbortError");
}

/** headersOf are the answer's headers, from the transfer's lines of them. */
function headersOf(xhr: Transfer): Headers {
  const headers = new Headers();
  for (const line of xhr.getAllResponseHeaders().split(/\r?\n/)) {
    const colon = line.indexOf(":");
    if (colon > 0) {
      headers.append(line.slice(0, colon).trim(), line.slice(colon + 1).trim());
    }
  }
  return headers;
}
