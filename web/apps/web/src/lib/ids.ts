/**
 * idPattern is an id the API gives, a UUID, in either case: anything else
 * in an address names nothing, and is not read, as the server would refuse
 * it as malformed (M8/P2B review A-N1): a trash item's node, a changeset,
 * an account or a page the history is filtered by (M8/P3 design 4.2, 4.3).
 */
export const idPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/iu;
