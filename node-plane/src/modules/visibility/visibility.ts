import { NatsConnection } from "@nats-io/transport-node";
import { IUser } from "../../types/user";
import {
  VisibilityBatchQuery,
  VisibilityBatchResponse,
} from "../../../gen/spatial/v1/spatial";
import { requestFullSync } from "../../simulation/SimulationEngine";

export const createVisibilityPinger = (
  nats: NatsConnection,
  users: IUser[],
  getCurrentHash: () => bigint,
  checkHashInHistory: (h: bigint) => boolean,
) => {
  const interval = setInterval(async () => {
    if (users.length === 0) return;

    const searchRadius = 150.0;
    const batchSize = Math.min(users.length, 50);
    const queries = [];

    for (let i = 0; i < batchSize; i++) {
      const randomUser = users[Math.floor(Math.random() * users.length)];
      queries.push({
        userId: randomUser.id,
        radius: searchRadius,
      });
    }

    const currentLocalHash = getCurrentHash();
    const queryMsg = {
      queries,
    };

    try {
      const response = await nats.request(
        "spatial.query.visibility",
        VisibilityBatchQuery.encode(queryMsg).finish(),
        { timeout: 500 },
      );

      const decoded = VisibilityBatchResponse.decode(response.data);
      const remoteHash = BigInt(decoded.stateHash || "0");

      if (!checkHashInHistory(remoteHash)) {
        console.error(
          `[Sync] Hash history miss. Desync confirmed! Remote: ${remoteHash.toString()} | Current Local: ${currentLocalHash.toString()}`,
        );
        requestFullSync();
      }
    } catch (err) {
      console.error("[Visibility] RPC failure:", err);
    }
  }, 1000);

  return {
    cleanup: () => {
      clearInterval(interval);
    },
  };
};
