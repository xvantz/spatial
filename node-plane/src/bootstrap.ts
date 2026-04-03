import { createNatsConnection } from "./infra/nats";
import { generateFakeUsers } from "./modules/generator/generateFakeUsers";
import { createListener } from "./modules/listener/listener";
import { createVisibilityPinger } from "./modules/visibility/visibility";

const bootstrap = async () => {
  try {
    console.log("[bootstrap] init plane system");

    const nats = await createNatsConnection();
    console.log("[bootstrap] NATS connected.");

    const fakeUsers = generateFakeUsers(1000);
    console.log("[bootstrap] generated fake users.");

    const listener = createListener(nats, fakeUsers);
    console.log("[bootstrap] listeners started.");

    const visibility = createVisibilityPinger(
      nats,
      fakeUsers,
      listener.getCurrentHash,
      listener.checkHashInHistory,
    );
    console.log("[bootstrap] visibility pinger started.");

    const shutdown = async (signal: string) => {
      console.log(`[Shutdown] Getted ${signal}. Stopped process...`);

      try {
        listener.cleanup();
        visibility.cleanup();

        await nats.drain();
        await nats.close();
        console.log("[Shutdown] NATS closed.");

        process.exit(0);
      } catch (e) {
        console.error("[Shutdown] Error closed connection:", e);
        process.exit(1);
      }
    };

    process.on("SIGTERM", () => shutdown("SIGTERM"));
    process.on("SIGINT", () => shutdown("SIGINT"));
  } catch (e) {
    console.error("[Fatal Error] Error started service:", e);
  }
};

process.on("unhandledRejection", (reason) => {
  console.error("[Unhandled Rejection]:", reason);
});

process.on("uncaughtException", (error) => {
  console.error("[Uncaught Exception]:", error);
});

bootstrap();
