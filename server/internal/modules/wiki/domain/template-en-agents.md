# How this LLM Wiki is kept

This notebook is an LLM Wiki: people add sources, and agents write and keep the wiki's pages from them. This page is the agreement both follow. An agent reads it when it connects, before it writes anything.

## Pages

Every page outside the source area is a wiki page about one thing: a concept, a person, an event, the summary of a source. Give it frontmatter with three keys:

```yaml
---
type: concept
summary: One sentence that says what the page is about.
sources: ["[[raw/Some article]]"]
---
```

- `type` puts the page in a group of the index. Use few types, and the same words each time, such as `concept`, `person`, `event` and `summary`.
- `summary` is one sentence. The index shows it next to the title.
- `sources` lists the sources the page draws on, each a link in quotes. Without the quotes, `sources: [[raw/Some article]]` is a nested list in YAML, not a link, and cites nothing.

Where pages mention each other, link them with `[[Title]]`, so that no page is left without a link to it.

## Sources

The page `raw` and everything under it is the source area. A source is added once, as a page or an attachment, and never changed after: to correct one, add a new source. Only the notebook's admins can rename, move or delete sources. A new source that is byte for byte one already there is refused, with the path of the one there.

## Generated pages

At the root, `index.md` and `log.md` are not pages anyone writes: the server writes them each time they are read. `index.md` lists the wiki's pages by type with their summaries; `log.md` lists the latest changes. No page or attachment at the root may be named `index` or `log`.

## Working with an agent

Connect an agent to this notebook at `/mcp?notebook=<id>`. Three prompts are served there:

- `ingest` reads a source, then writes or updates the pages it touches, in one change.
- `query` answers a question from the wiki's pages, citing them and their sources.
- `lint` checks the wiki and fixes what can be fixed without asking.

## This page

While the notebook is an LLM Wiki, this page cannot be deleted, renamed or moved. Its text is yours: rewrite it for your field, your types and the way you work.
