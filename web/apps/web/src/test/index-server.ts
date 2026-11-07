import type { NotebookIndex } from "@nervewiki/api-client";

import { json, notebookJSON, type Answer } from "./fakes";
import { guide, install, linux, notes, pageNode, pageServer } from "./page-server";

export const indexHome = `/lab/notebooks/${notebookJSON.id}`;
export const indexPath = `${indexHome}/index`;
const indexRoute = `GET /api/v0/notebooks/${notebookJSON.id}/index`;
export const indexRaw = pageNode(51, "raw", guide);
export const firstId = "0199a2b4-0000-7000-8000-000000000701";
export const secondId = "0199a2b4-0000-7000-8000-000000000702";
export const thirdId = "0199a2b4-0000-7000-8000-000000000703";

/** Complete wire literals; neither grouping nor type normalization is computed by this fake. */
export function indexFacts(): NotebookIndex {
  return {
    data: [
      {
        page_id: firstId,
        parent_id: null,
        title: "Alpha",
        type: "Note",
        type_key: "note",
        summary: "First summary",
        summary_cut: false,
        tags: ["plan", "plan/q4"],
        updated_at: "2026-10-03T08:00:00Z",
        inbound: 2,
        inbound_capped: false,
      },
      {
        page_id: secondId,
        parent_id: guide.id,
        title: "Bravo",
        type: "note",
        type_key: "note",
        summary: "A shortened summary",
        summary_cut: true,
        tags: [],
        updated_at: "2026-10-04T08:00:00Z",
        inbound: 1000,
        inbound_capped: true,
      },
      {
        page_id: thirdId,
        parent_id: notes.id,
        title: "Charlie",
        type: "Topic",
        type_key: "topic",
        summary: null,
        summary_cut: false,
        tags: ["reference"],
        updated_at: "2026-10-05T08:00:00Z",
        inbound: 0,
        inbound_capped: false,
      },
    ],
    next_cursor: null,
    types: [
      { type: "Note", count: 1 },
      { type: "note", count: 1 },
      { type: "Topic", count: 1 },
    ],
    types_complete: true,
    unfilterable_type_pages: 0,
    source_area: null,
    mode: "standard",
  };
}

/** Only the transport is faked. Actual routes, services, stores, SWR and components remain in use. */
export function indexServer(answers: Record<string, Answer> = {}) {
  const fixture = {
    value: indexFacts(),
    requests: [] as Request[],
    answer: undefined as Answer | undefined,
  };
  const server = pageServer({
    role: "reader",
    nodes: [guide, install, linux, notes, indexRaw],
    answers: {
      ...answers,
      [indexRoute]: (request) => {
        fixture.requests.push(request);
        return fixture.answer === undefined ? json(fixture.value) : fixture.answer(request);
      },
    },
  });
  return {
    requests: fixture.requests,
    get value() {
      return fixture.value;
    },
    set value(value: NotebookIndex) {
      fixture.value = value;
    },
    get answer() {
      return fixture.answer;
    },
    set answer(value: Answer | undefined) {
      fixture.answer = value;
    },
    server,
  };
}
