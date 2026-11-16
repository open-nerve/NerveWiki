import { appendFileSync, mkdirSync } from "node:fs";
import path from "node:path";

// Where a round of perf writes (M12/P4 design 4.1): one JSON object a line, appended, so that two rounds, a check run
// again or a control land beside each other when NW_PERF_OUT names one file for them; the serve's log, in the same
// directory, then holds the last run's only (e2e/fixtures/server.ts opens it anew). Playwright empties its outputDir
// as a run starts, before the setup: the default file holds this run's lines only.

/** The file the lines go to: NW_PERF_OUT, or results.jsonl in perf's outputDir. */
function resultsFile(): string {
  return path.resolve(process.env.NW_PERF_OUT ?? path.join(import.meta.dirname, "../test-results/perf/results.jsonl"));
}

/** The round's directory, the results file's: its serve's log and storage, its key, and perf-env.json. */
export function roundDir(): string {
  return path.dirname(resultsFile());
}

/** Appends a line of what, with data, and when it was written. */
export function record(what: string, data: Record<string, unknown>): void {
  mkdirSync(roundDir(), { recursive: true });
  appendFileSync(resultsFile(), `${JSON.stringify({ what, at: new Date().toISOString(), ...data })}\n`);
}

/** The median of values by nearest rank: the middle one of an odd count, the lower middle of an even one. */
export function median(values: readonly number[]): number {
  const sorted = values.toSorted((a, b) => a - b);
  const value = sorted[Math.ceil(sorted.length / 2) - 1];
  if (value === undefined) {
    throw new Error("the median of no values");
  }
  return value;
}
