import { IUser } from "../types/user";

const WORLD = { SIZE_X: 3500, SIZE_Y: 3500, SIZE_Z: 1000 } as const;
const SPEED_PER_SEC = 375;

let fullSyncRequested = true;

export const requestFullSync = (): void => {
  fullSyncRequested = true;
};

/**
 * SimulationEngine handles entity movement simulation independently
 * of hash tracking or NATS publishing. Each tick returns the set of
 * users that moved (with positions already updated in-place).
 */
export interface ISimulationEngine {
  /** Advance one simulation tick. Returns users whose positions changed. */
  tick(dt: number): IUser[];

  /** Return all simulated users (the full world state). */
  getAllUsers(): IUser[];

  /** Clean up internal state. */
  cleanup(): void;
}

export function createSimulationEngine(users: IUser[]): ISimulationEngine {
  const dirtySet = new Set<number>();

  const tick = (dt: number): IUser[] => {
    const batchToSync: IUser[] = [];

    if (fullSyncRequested) {
      console.warn("[Simulation] Full world synchronization triggered");
      fullSyncRequested = false;
      batchToSync.push(...users);
    } else {
      const updateCount = Math.floor(Math.random() * 51) + 50;
      while (dirtySet.size < updateCount) {
        dirtySet.add(Math.floor(Math.random() * users.length));
      }

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

        u.position.x = Math.fround(u.position.x);
        u.position.y = Math.fround(u.position.y);
        u.position.z = Math.fround(u.position.z);

        batchToSync.push(u);
      }
      dirtySet.clear();
    }

    return batchToSync;
  };

  const getAllUsers = (): IUser[] => users;

  const cleanup = (): void => {
    dirtySet.clear();
  };

  return { tick, getAllUsers, cleanup };
}
