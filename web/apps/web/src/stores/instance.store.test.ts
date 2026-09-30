import { expect, test } from "vitest";

import { instanceJSON } from "../test/fakes";
import { InstanceStore } from "./instance.store";

test("load keeps what the service answers", async () => {
  const store = new InstanceStore({ get: async () => instanceJSON });

  await store.load();

  expect(store.info).toEqual(instanceJSON);
});

test("a failed load throws and keeps what the store had", async () => {
  let fail = false;
  const store = new InstanceStore({
    get: async () => {
      if (fail) {
        throw new Error("down");
      }
      return instanceJSON;
    },
  });
  await store.load();
  fail = true;

  await expect(store.load()).rejects.toThrow("down");
  expect(store.info).toEqual(instanceJSON);
});
