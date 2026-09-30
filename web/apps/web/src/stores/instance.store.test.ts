import { expect, test } from "vitest";

import { instanceJSON } from "../test/fakes";
import { InstanceStore } from "./instance.store";

test("fetch keeps what the service answers", async () => {
  const store = new InstanceStore({ get: async () => instanceJSON });

  await store.fetch();

  expect(store.info).toEqual(instanceJSON);
});

test("a failed fetch throws and keeps what the store had", async () => {
  let fail = false;
  const store = new InstanceStore({
    get: async () => {
      if (fail) {
        throw new Error("down");
      }
      return instanceJSON;
    },
  });
  await store.fetch();
  fail = true;

  await expect(store.fetch()).rejects.toThrow("down");
  expect(store.info).toEqual(instanceJSON);
});
