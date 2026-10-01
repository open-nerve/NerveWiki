import { useSWRConfig } from "swr";

import { ApiError } from "../services/api";

/**
 * useFollowRole is the onError of a read that only a workspace's admins
 * may make: refused as forbidden, the account is no longer one, which
 * another admin changed elsewhere; it reads the workspaces again, whose
 * roles the pages follow (M3/P5 review M1).
 */
export function useFollowRole(): (error: unknown) => void {
  const { mutate } = useSWRConfig();
  return (error) => {
    if (error instanceof ApiError && error.code === "forbidden") {
      void mutate("workspaces");
    }
  };
}
