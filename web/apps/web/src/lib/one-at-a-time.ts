/**
 * A queue that runs the tasks given to it one at a time, in the order given: each starts once the one before it
 * has settled, whether it succeeded or failed. A store sends its changes of one resource through one, so the
 * server applies the changes in the order they were made, and the last change answered is what the server holds
 * (M1/P5 design 3.3). A change that failed without an answer (the connection broke after the request reached
 * the server) may still be applied after the next one. A task that never settles holds the queue: the server
 * answers every request it receives within its server.request_timeout.
 */
export function oneAtATime(): <T>(task: () => Promise<T>) => Promise<T> {
  let last: Promise<unknown> = Promise.resolve();
  return <T>(task: () => Promise<T>): Promise<T> => {
    const run = last.then(task);
    last = run.catch(() => undefined);
    return run;
  };
}
