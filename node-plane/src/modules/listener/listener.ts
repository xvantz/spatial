import { NatsConnection } from "@nats-io/transport-node";
import { IUser } from "../../types/user";
import { TelemetryBatch } from "../../../gen/spatial/v1/spatial";

const WORLD = { SIZE_X: 3500, SIZE_Y: 3500, SIZE_Z: 1000 } as const;
const SPEED = 15;

export const createListener = (nats: NatsConnection, users: IUser[]) => {
  const dirtySet = new Set<number>();

  const sub = nats.subscribe("engine.tick", {
    callback: (err, _) => {
      if (err) return console.error("[NATS] Error in engine.tick", err);

      const updateCount = Math.floor(Math.random() * 51) + 50;

      while (dirtySet.size < updateCount) {
        dirtySet.add(Math.floor(Math.random() * users.length));
      }

      const batchToSync: IUser[] = [];

      for (const idx of dirtySet) {
        const u = users[idx];

        u.position.x += (Math.random() * 2 - 1) * SPEED;
        u.position.y += (Math.random() * 2 - 1) * SPEED;
        u.position.z += (Math.random() * 2 - 1) * SPEED;

        if (u.position.x > WORLD.SIZE_X) u.position.x = WORLD.SIZE_X;
        if (u.position.x < -WORLD.SIZE_X) u.position.x = -WORLD.SIZE_X;

        if (u.position.y > WORLD.SIZE_Y) u.position.y = WORLD.SIZE_Y;
        if (u.position.y < -WORLD.SIZE_Y) u.position.y = -WORLD.SIZE_Y;

        if (u.position.z > WORLD.SIZE_Z) u.position.z = WORLD.SIZE_Z;
        if (u.position.z < -WORLD.SIZE_Z) u.position.z = -WORLD.SIZE_Z;

        batchToSync.push(u);
      }
      dirtySet.clear();

      console.log(`[Tick] Moved ${batchToSync.length} users.`);

      const telemetryMessage: TelemetryBatch = {
        players: batchToSync.map((u) => ({
          userId: u.id,
          position: u.position,
        })),
      };
      const payload = TelemetryBatch.encode(telemetryMessage).finish();

      nats.publish("spatial.telemetry", payload);
    },
  });

  return {
    cleanup: () => {
      console.log("[Listener] Stoping subscribe and clearing state...");
      sub.unsubscribe();
      dirtySet.clear();
    },
  };
};
