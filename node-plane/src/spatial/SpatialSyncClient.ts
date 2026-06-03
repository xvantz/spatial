import { NatsConnection, Subscription } from "@nats-io/transport-node";
import { IUser } from "../types/user";
import { TelemetryBatch, PlayerRemove, Heartbeat } from "../../gen/spatial/v1/spatial";
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

  /**
   * Subscribe to handshake requests from the Go worker.
   * When a sync_request arrives, responds with a full state snapshot.
   */
  setupHandshake(getAllUsers: () => IUser[]): void;

  /**
   * Subscribe to heartbeat messages from the Go worker and track liveness.
   */
  setupHeartbeat(): void;

  /**
   * Remove a player from the tracked hash state and notify the Go worker.
   * XORs the player's current hash out of the global totalHash.
   */
  removePlayer(userId: number): void;

  /** Clean up internal state. */
  cleanup(): void;
}

export function createSpatialSyncClient(nats: NatsConnection): ISpatialSyncClient {
  let totalHash = BigInt(0);
  const playerHashes = new Map<number, bigint>();
  const hashHistory = new Set<bigint>();
  const maxHistorySize = 1000;
  let handshakeSub: Subscription | null = null;
  let heartbeatSub: Subscription | null = null;
  let heartbeatInterval: ReturnType<typeof setInterval> | null = null;
  let lastHeartbeat = 0;

  const HEARTBEAT_TIMEOUT_MS = 15_000;

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

  const setupHandshake = (getAllUsers: () => IUser[]): void => {
    if (handshakeSub) return; // already registered

    handshakeSub = nats.subscribe("spatial.handshake.sync", {
      callback: (_err, msg) => {
        const allUsers = getAllUsers();
        const telemetry: TelemetryBatch = {
          players: allUsers.map((u) => ({
            userId: u.id,
            position: u.position,
          })),
          stateHash: totalHash.toString(),
        };
        msg.respond(TelemetryBatch.encode(telemetry).finish());
        console.log(
          `[Handshake] Responded with ${allUsers.length} players (hash=${totalHash})`,
        );
      },
    });
  };

  const setupHeartbeat = (): void => {
    if (heartbeatSub) return;

    heartbeatSub = nats.subscribe("spatial.health.heartbeat", {
      callback: (_err, msg) => {
        const heartbeat = Heartbeat.decode(msg.data);
        lastHeartbeat = Number(heartbeat.timestampMs);
      },
    });

    // Periodic staleness check every 10s
    heartbeatInterval = setInterval(() => {
      const elapsed = Date.now() - lastHeartbeat;
      if (elapsed > HEARTBEAT_TIMEOUT_MS && lastHeartbeat > 0) {
        console.warn(
          `[Heartbeat] No heartbeat for ${Math.floor(elapsed / 1000)}s — worker may be down`,
        );
      }
    }, 10_000);
  };

  const removePlayer = (userId: number): void => {
    const oldHash = playerHashes.get(userId);
    if (oldHash === undefined) return;

    totalHash ^= oldHash;
    playerHashes.delete(userId);

    const msg: PlayerRemove = { userId };
    nats.publish("spatial.player.remove", PlayerRemove.encode(msg).finish());
  };

  const cleanup = (): void => {
    if (handshakeSub) {
      handshakeSub.unsubscribe();
      handshakeSub = null;
    }
    if (heartbeatSub) {
      heartbeatSub.unsubscribe();
      heartbeatSub = null;
    }
    if (heartbeatInterval) {
      clearInterval(heartbeatInterval);
      heartbeatInterval = null;
    }
    playerHashes.clear();
    hashHistory.clear();
  };

  return { getCurrentHash, checkHashInHistory, sync, setupHandshake, setupHeartbeat, removePlayer, cleanup };
}
