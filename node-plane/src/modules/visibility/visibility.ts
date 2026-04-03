import { NatsConnection } from "@nats-io/transport-node";
import { IUser } from "../../types/user";
import {
  VisibilityBatchQuery,
  VisibilityBatchResponse,
} from "../../../gen/spatial/v1/spatial";
import { requestFullSync } from "../listener/listener";

export const createVisibilityPinger = (
  nats: NatsConnection,
  users: IUser[],
  getCurrentHash: () => bigint,
  checkHashInHistory: (h: bigint) => boolean,
) => {
  console.log("[Pinger] Start polling visibility");

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
      expectedStateHash: currentLocalHash.toString(),
    };

    const payload = VisibilityBatchQuery.encode(queryMsg).finish();

    try {
      const start = performance.now();
      const response = await nats.request("spatial.query.visibility", payload, {
        timeout: 500,
      });

      const decoded = VisibilityBatchResponse.decode(response.data);
      const elapsed = performance.now() - start;

      const remoteHash = BigInt(decoded.stateHash || "0");

      // Check if remote hash exists in our recent history
      if (!checkHashInHistory(remoteHash)) {
        console.error(
          `[DESYNC] Remote hash ${remoteHash.toString()} not found in local history! Current Local: ${currentLocalHash.toString()}`,
        );
        requestFullSync();
      }

      console.log(
        `[CQRS] batch size ${batchSize} users processed for ${elapsed.toFixed(2)}ms`,
      );

      if (decoded.results && decoded.results.length > 0) {
        const first = decoded.results[0];

        const visibleCount = first.visibleUserIds
          ? first.visibleUserIds.length
          : 0;

        console.log(
          `       User ${first.userId} sees around self (${searchRadius}m): ${visibleCount} entities.`,
        );

        if (visibleCount > 0) {
          console.log(
            `       ID neighbors: ${first.visibleUserIds.join(", ")}`,
          );
        }
      }
    } catch (err) {
      console.error("[CQRS] Error or timeout batch request visibility", err);
    }
  }, 1000);

  return {
    cleanup: () => {
      console.log("[Pinger] Stop polling...");
      clearInterval(interval);
    },
  };
};
