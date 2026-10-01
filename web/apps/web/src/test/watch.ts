/**
 * watchFor watches the page for text being added to it; the function it
 * returns stops watching, and says whether it was.
 */
export function watchFor(text: string): () => boolean {
  let seen = false;
  const look = (records: MutationRecord[]) =>
    (seen ||= records.some((record) => [...record.addedNodes].some((node) => node.textContent?.includes(text))));
  const observer = new MutationObserver(look);
  observer.observe(document.body, { childList: true, subtree: true });
  return () => {
    look(observer.takeRecords());
    observer.disconnect();
    return seen;
  };
}
