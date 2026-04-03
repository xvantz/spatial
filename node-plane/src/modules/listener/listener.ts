import { NatsConnection } from "@nats-io/transport-node";
import { IUser } from "../../types/user";
import { TelemetryBatch } from "../../../gen/spatial/v1/spatial";
import { hashPlayer } from "../generator/hash";

const WORLD = { SIZE_X: 3500, SIZE_Y: 3500, SIZE_Z: 1000 } as const;
const SPEED_PER_SEC = 375;
const TICK_RATE_MS = 40;

let fullSyncRequested = true;

export const requestFullSync = () => {
  fullSyncRequested = true;
};

export const createListener = (nats: NatsConnection, users: IUser[]) => {
  const dirtySet = new Set<number>();
  let lastTickTime = performance.now();

  let totalHash = BigInt(0);
  const playerHashes = new Map<number, bigint>();
  const hashHistory = new Set<bigint>();

  for (const u of users) {
    u.position.x = Math.fround(u.position.x);
    u.position.y = Math.fround(u.position.y);
    u.position.z = Math.fround(u.position.z);
    const h = hashPlayer(u.id, u.position.x, u.position.y, u.position.z);
    playerHashes.set(u.id, h);
    totalHash ^= h;
  }

  hashHistory.add(totalHash);

  const interval = setInterval(() => {
    const now = performance.now();
    const dt = (now - lastTickTime) / 1000;
    lastTickTime = now;

    const batchToSync: IUser[] = [];

    if (fullSyncRequested) {
      console.warn("[Telemetry] Full world synchronization triggered");
      fullSyncRequested = false;
      batchToSync.push(...users);
    } else {
      const updateCount = Math.floor(Math.random() * 51) + 50;
      while (dirtySet.size < updateCount) {
        dirtySet.add(Math.floor(Math.random() * users.length));
      }

      for (const idx of dirtySet) {
        const u = users[idx];
        const oldHash = playerHashes.get(u.id) || BigInt(0);
        totalHash ^= oldHash;

        u.position.x += (Math.random() * 2 - 1) * SPEED_PER_SEC * dt;
        u.position.y += (Math.random() * 2 - 1) * SPEED_PER_SEC * dt;
        u.position.z += (Math.random() * 2 - 1) * SPEED_PER_SEC * dt;

        if (u.position.x > WORLD.SIZE_X) u.position.x = WORLD.SIZE_X;
        if (u.position.x < -WORLD.SIZE_X) u.position.x = -WORLD.SIZE_X;
        if (u.position.y > WORLD.SIZE_Y) u.position.y = WORLD.SIZE_Y;
        if (u.position.y < -WORLD.SIZE_Y) u.position.y = -WORLD.SIZE_Y;
        if (u.position.z > WORLD.SIZE_Z) u.position.z = WORLD.SIZE_Z;
        if (u.position.z < -WORLD.SIZE_Z) u.position.z = -WORLD.SIZE_Z;

        u.position.x = Math.fround(u.position.x);
        u.position.y = Math.fround(u.position.y);
        u.position.z = Math.fround(u.position.z);

        const newHash = hashPlayer(u.id, u.position.x, u.position.y, u.position.z);
        playerHashes.set(u.id, newHash);
        totalHash ^= newHash;

        batchToSync.push(u);
      }
      dirtySet.clear();
    }

    hashHistory.add(totalHash);
    if (hashHistory.size > 100) {
      const first = hashHistory.values().next().value;
      if (first !== undefined) hashHistory.delete(first);
    }

    const telemetryMessage: TelemetryBatch = {
      players: batchToSync.map((u) => ({
        userId: u.id,
        position: u.position,
      })),
      stateHash: totalHash.toString(),
    };

    nats.publish("spatial.telemetry", TelemetryBatch.encode(telemetryMessage).finish());
  }, TICK_RATE_MS);

  return {
    getCurrentHash: () => totalHash,
    checkHashInHistory: (h: bigint) => hashHistory.has(h),
    cleanup: () => {
      clearInterval(interval);
      dirtySet.clear();
      hashHistory.clear();
    },
  };
};
