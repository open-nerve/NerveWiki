import { createDatabase, startPostgres, templateDatabase } from "./fixtures/db";
import { requireBinary, runNervewiki } from "./fixtures/server";

/**
 * Starts PostgreSQL once per run and migrates the template database with
 * bin/nervewiki migrate up; every worker copies the template (fixtures/test.ts).
 * The returned function is the global teardown.
 */
export default async function globalSetup(): Promise<() => Promise<void>> {
  requireBinary();
  const postgres = await startPostgres();
  await runNervewiki(["migrate", "up"], await createDatabase(templateDatabase));
  return postgres.stop;
}
