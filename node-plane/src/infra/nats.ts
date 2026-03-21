import { connect, NatsConnection } from "@nats-io/transport-node";

let nc: NatsConnection;

export const createNatsConnection = async () => {
  if (!nc) {
    nc = await connect({
      servers: "nats://localhost:4222",
      reconnect: true,
      maxReconnectAttempts: -1,
      waitOnFirstConnect: true,
    });
  }

  return nc;
};
