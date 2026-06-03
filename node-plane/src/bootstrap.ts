import { createNatsConnection } from "./infra/nats";
import { generateFakeUsers } from "./modules/generator/generateFakeUsers";
import { createSpatialSyncClient } from "./spatial/SpatialSyncClient";
import { createSimulationEngine } from "./simulation/SimulationEngine";
import { createVisibilityPinger } from "./modules/visibility/visibility";

const TICK_RATE_MS = 40;

const bootstrap = async () => {
  try {
    console.log("[bootstrap] init plane system");

    const nats = await createNatsConnection();
    console.log("[bootstrap] NATS connected.");

    const fakeUsers = generateFakeUsers(1000);
    console.log("[bootstrap] generated fake users.");

    const syncClient = createSpatialSyncClient(nats);
    const simulation = createSimulationEngine(fakeUsers);

    // Initialise hashes for all users
    syncClient.sync(fakeUsers);
    console.log("[bootstrap] hashes initialized.");

    // Run simulation in a fixed tick loop
    let lastTickTime = performance.now();
    const interval = setInterval(() => {
      const now = performance.now();
      const dt = (now - lastTickTime) / 1000;
      lastTickTime = now;

      const moved = simulation.tick(dt);
      syncClient.sync(moved);
    }, TICK_RATE_MS);

    const visibility = createVisibilityPinger(
      nats,
      fakeUsers,
      syncClient.getCurrentHash,
      syncClient.checkHashInHistory,
    );
    console.log("[bootstrap] visibility pinger started.");

    const shutdown = async (signal: string) => {
      console.log(`[Shutdown] Getted ${signal}. Stopped process...`);

      try {
        clearInterval(interval);
        simulation.cleanup();
        syncClient.cleanup();
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
