// Only type-checked (pnpm check:types), never run: the generated types must
// describe the contract, so that a wrong call fails to compile. TypeScript
// reports an expect-error directive whose line compiles, so each one below
// proves that its line fails.
import { createClient, type InstanceInfo, type Problem } from "../src/index";

const client = createClient({ baseUrl: "http://localhost:8080" });

export async function describeInstance(): Promise<InstanceInfo> {
  const { data, error } = await client.GET("/api/v0/instance");
  if (error) {
    const problem: Problem = error;
    throw new Error(problem.code);
  }
  const apiVersion: "v0" = data.api_version;
  void apiVersion;
  return data;
}

export async function wrongCalls(): Promise<void> {
  // @ts-expect-error: the contract has no such path
  await client.GET("/api/v0/nope");
  // @ts-expect-error: the instance is read-only
  await client.POST("/api/v0/instance");
}
