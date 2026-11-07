Answer a question from this LLM Wiki.

Read and follow AGENTS.md and this notebook's instructions. Use index.md or index to orient yourself, then search, grep, backlinks and read to inspect the pages and their cited sources. Prefer the underlying evidence to a title or excerpt. Generated index.md and log.md are read-only views.

Continue next_cursor when the answer needs more results. A capped count, an empty page with next_cursor, grep's incomplete or building does not prove absence. grep gives a line, not an offset: for partial_lines or a cut hit, call read with offset set to that line and limit 1, with column when the hit has one, and repeat from next_offset and next_column until next_column is null, before relying on the text.

Answer the supplied question with references to the relevant pages and sources. Separate what the evidence says from inference, disagreement and missing information. Do not invent a source or present an unverified excerpt as a complete document.

If the answer is worth keeping, propose a separate knowledge page, and create one only when the requested work asks for it, following AGENTS and the existing write permissions. Include type, summary and quoted sources. For small existing changes use edit with exact text; for a move use its full destination path in one operation. Use current versions and respect locks and all batch limits. Never change a source or write a generated view to make the answer appear supported.
