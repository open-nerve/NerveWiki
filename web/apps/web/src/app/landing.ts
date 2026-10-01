/**
 * landingPath is where / goes (M2/P5 design 3.3): the workspace this device
 * showed last, if the account still has it; else the first of the list,
 * which the server sorts by name; else the page that creates one, which
 * says what to do instead while creation is off.
 */
export function landingPath(workspaces: readonly { slug: string }[], last: string | undefined): string {
  const slug = workspaces.find((workspace) => workspace.slug === last)?.slug ?? workspaces[0]?.slug;
  return slug === undefined ? "/create-workspace" : `/${slug}`;
}
