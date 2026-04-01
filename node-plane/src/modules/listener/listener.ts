import { NatsConnection } from "@nats-io/transport-node";
import { IUser } from "../../types/user";
import { TelemetryBatch } from "../../../gen/spatial/v1/spatial";

const WORLD = { SIZE_X: 3500, SIZE_Y: 3500, SIZE_Z: 1000 } as const;
const SPEED_PER_SEC = 375;
const TICK_RATE_MS = 40;

export const createListener = (nats: NatsConnection, users: IUser[]) => {
  const dirtySet = new Set<number>();

  let ticks = 0;
  let lastReportTime = performance.now();
  let lastTickTime = performance.now();

  console.log(
    `[Telemetry] Started generator. Target: ${1000 / TICK_RATE_MS} TPS`,
  );

  const interval = setInterval(() => {
    const now = performance.now();
    const dt = (now - lastTickTime) / 1000;
    lastTickTime = now;

    ticks++;

    if (now - lastReportTime >= 1000) {
      const elapsed = (now - lastReportTime) / 1000;
      const tps = (ticks / elapsed).toFixed(2);
      console.log(
        `[Heartbeat] Real TPS: ${tps} | Last dt: ${(dt * 1000).toFixed(2)}ms`,
      );

      ticks = 0;
      lastReportTime = now;
    }

    const updateCount = Math.floor(Math.random() * 51) + 50;

    while (dirtySet.size < updateCount) {
      dirtySet.add(Math.floor(Math.random() * users.length));
    }

    const batchToSync: IUser[] = [];

    for (const idx of dirtySet) {
      const u = users[idx];

      u.position.x += (Math.random() * 2 - 1) * SPEED_PER_SEC * dt;
      u.position.y += (Math.random() * 2 - 1) * SPEED_PER_SEC * dt;
      u.position.z += (Math.random() * 2 - 1) * SPEED_PER_SEC * dt;

      if (u.position.x > WORLD.SIZE_X) u.position.x = WORLD.SIZE_X;
      if (u.position.x < -WORLD.SIZE_X) u.position.x = -WORLD.SIZE_X;

      if (u.position.y > WORLD.SIZE_Y) u.position.y = WORLD.SIZE_Y;
      if (u.position.y < -WORLD.SIZE_Y) u.position.y = -WORLD.SIZE_Y;

      if (u.position.z > WORLD.SIZE_Z) u.position.z = WORLD.SIZE_Z;
      if (u.position.z < -WORLD.SIZE_Z) u.position.z = -WORLD.SIZE_Z;

      batchToSync.push(u);
    }
    dirtySet.clear();

    const telemetryMessage: TelemetryBatch = {
      players: batchToSync.map((u) => ({
        userId: u.id,
        position: u.position,
      })),
    };

    const payload = TelemetryBatch.encode(telemetryMessage).finish();
    nats.publish("spatial.telemetry", payload);
  }, TICK_RATE_MS);

  return {
    cleanup: () => {
      console.log("[Listener] Stoping generator moves...");
      clearInterval(interval);
      dirtySet.clear();
    },
  };
};
