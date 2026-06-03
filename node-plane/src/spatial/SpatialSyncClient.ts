import { NatsConnection } from "@nats-io/transport-node";
import { IUser } from "../types/user";
import { TelemetryBatch } from "../../gen/spatial/v1/spatial";
import { hashPlayer } from "../modules/generator/hash";

/**
 * SpatialSyncClient manages the commutative XOR hash of the world state
 * and publishes position telemetry to NATS. It has no knowledge of the
 * simulation layer — it only tracks hashes and publishes updates.
 */
export interface ISpatialSyncClient {
  /** Get the current global XOR hash of all entity positions. */
  getCurrentHash(): bigint;

  /** Check if a given hash exists in the sliding history window. */
  checkHashInHistory(h: bigint): boolean;

  /**
   * Publish telemetry for moved users and update internal hashes.
   * Each user's current position is used to compute the new hash.
   */
  sync(users: IUser[]): void;

  /** Clean up internal state. */
  cleanup(): void;
}

export function createSpatialSyncClient(nats: NatsConnection): ISpatialSyncClient {
  let totalHash = BigInt(0);
  const playerHashes = new Map<number, bigint>();
  const hashHistory = new Set<bigint>();
  const maxHistorySize = 100;

  const getCurrentHash = (): bigint => totalHash;

  const checkHashInHistory = (h: bigint): boolean => hashHistory.has(h);

  const sync = (users: IUser[]): void => {
    if (users.length === 0) return;

    for (const u of users) {
      const oldHash = playerHashes.get(u.id) || BigInt(0);
      totalHash ^= oldHash;

      const newHash = hashPlayer(u.id, u.position.x, u.position.y, u.position.z);
      playerHashes.set(u.id, newHash);
      totalHash ^= newHash;
    }

    hashHistory.add(totalHash);
    if (hashHistory.size > maxHistorySize) {
      const first = hashHistory.values().next().value;
      if (first !== undefined) hashHistory.delete(first);
    }

    const telemetryMessage: TelemetryBatch = {
      players: users.map((u) => ({
        userId: u.id,
        position: u.position,
      })),
      stateHash: totalHash.toString(),
    };

    nats.publish("spatial.telemetry", TelemetryBatch.encode(telemetryMessage).finish());
  };

  const cleanup = (): void => {
    playerHashes.clear();
    hashHistory.clear();
  };

  return { getCurrentHash, checkHashInHistory, sync, cleanup };
}
