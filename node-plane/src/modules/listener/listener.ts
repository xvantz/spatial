import { NatsConnection } from "@nats-io/transport-node";
import { IUser } from "../../types/user";

export const createListener = (nats: NatsConnection, users: IUser[]) => {
  const dirtySet = new Set<number>();

  const sub = nats.subscribe("engine.tick", {
    callback: (err, msg) => {
      if (err) return console.error(err);
      dirtySet.clear();
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
