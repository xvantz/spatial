# HashGrid Spatial Engine

A high-performance distributed spatial partitioning system with incremental state synchronization between a Node.js Plane (Master) and a Go Coprocessor (Worker).

## Why This Architecture? Solving Node.js Bottlenecks

In high-load real-time applications (like MMOs or simulation engines), Node.js often faces significant performance challenges:

1.  **Single-Threaded Event Loop**: Node.js is excellent for I/O but struggles with CPU-bound tasks. Calculating visibility for 1,000+ entities every 40ms requires intensive mathematical operations (distance checks, bucket iterations). If performed on the main thread, this blocks the Event Loop, causing latency spikes and dropped ticks.
2.  **Expensive Spatial Math**: Heavy spatial partitioning logic in JavaScript often leads to high Garbage Collection (GC) pressure and unpredictable execution times.
3.  **The Solution**: By offloading the "heavy lifting" to a **Go Coprocessor**, we achieve:
    *   **True Parallelism**: Go's goroutines and `sync.RWMutex` allow multiple visibility queries to execute simultaneously across all CPU cores.
    *   **Predictable Performance**: Go provides native 32/64-bit float performance and manual control over memory (via `sync.Pool`), ensuring sub-millisecond query responses without blocking the Node.js Master.
    *   **Scalability**: The Node-Plane remains lightweight, focusing on high-level logic and I/O, while the Go-Worker scales the complex spatial math.

## System Architecture

The system consists of two main components communicating over **NATS** using **Protocol Buffers**:

1.  **Node-Plane (TypeScript/Node.js)**:
    *   Acts as the **Source of Truth** for entity positions.
    *   Simulates entity movement (40ms ticks).
    *   Calculates a **Commutative XOR Hash** of the entire world state.
    *   Maintains a **Hash History** to mitigate network latency in desync detection.

2.  **Go-Worker (Golang)**:
    *   Acts as a **Spatial Coprocessor**.
    *   Maintains a voxel-based **Spatial Grid** for efficient proximity queries ($O(1)$ bucket access).
    *   Provides high-speed visibility calculation services.
    *   Independently tracks the state hash to ensure data integrity.

## Synchronization Protocol

### 1. Commutative XOR Hashing
To ensure the Node-Plane and Go-Worker are always in sync without sending the full world state every tick, the system uses a commutative hash:
-   Each entity's state (ID + Position) is hashed using **FNV-1a (64-bit)**.
-   The global state hash is calculated as: `TotalHash = Hash(P1) ^ Hash(P2) ^ ... ^ Hash(Pn)`.
-   When an entity moves: `TotalHash = TotalHash ^ OldEntityHash ^ NewEntityHash`.
-   **Precision**: `Math.fround()` in JavaScript and `float32` in Go are used to ensure bit-identical representation of coordinates across runtimes.

### 2. Desync Detection & Mitigation
-   **Atomic Updates**: Go-Worker uses `BulkUpdate` under a single write lock to process telemetry batches, preventing "dirty reads" of the hash during query processing.
-   **Hash History**: Node-Plane maintains a sliding window of the last 100 generated hashes.
-   **Validation**: When a Visibility Query response arrives from Go, Node-Plane checks if the returned `StateHash` exists in its history.
-   **Recovery**: If the remote hash is not found in history, a **Full Synchronization** is triggered, dumping all entity positions to the Go-Worker in the next tick.

## Tech Stack

- **Transport**: [NATS](https://nats.io/) (Pub/Sub and Req/Rep)
- **Serialization**: [Protocol Buffers (v3)](https://protobuf.dev/)
- **Node-Plane**: Node.js (v20+), Bun, TypeScript
- **Go-Worker**: Go (v1.25+)
- **Protobuf Toolchain**: [Buf](https://buf.build/)

## Getting Started

### 1. Fast Track (Docker Compose)
The easiest way to run the entire system (NATS + Go-Worker + Node-Plane) is using Docker:
```bash
docker compose up --build
```
This will:
1.  Start the **NATS** broker.
2.  Build and run the **Go Spatial Worker**.
3.  Build and run the **Node Master Plane**.

### 2. Manual Development Setup

#### Infrastructure (NATS Only)
If you want to run services manually for debugging:
```bash
docker compose up nats -d
```

#### Go-Worker (Coprocessor)
Navigate to the `go-worker` directory and run the service:
```bash
cd go-worker
export NATS_URL=nats://localhost:4222
go run ./cmd/coprocessor/main.go
```

#### Node-Plane (Master)
Navigate to the `node-plane` directory, install dependencies, and start the simulation:
```bash
cd node-plane
pnpm install
export NATS_URL=nats://localhost:4222
pnpm run dev
```

## Messaging Schema (`spatial.proto`)

### Telemetry (Pub/Sub)
-   **Subject**: `spatial.telemetry`
-   **Payload**: `TelemetryBatch`
    *   `players`: List of moved entities (ID, X, Y, Z).
    *   `state_hash`: The global XOR hash after these moves.

### Proximity Query (Req/Rep)
-   **Subject**: `spatial.query.visibility`
-   **Payload**: `VisibilityBatchQuery` -> `VisibilityBatchResponse`
    *   Request contains entity IDs and their search radii.
    *   Response contains lists of visible neighbors and the current `state_hash` of the worker.

## Implementation Details

### Spatial Grid (Go)
The Go-Worker implements a voxel grid with a configurable `CellSize` (default: 50.0m).
-   **Bucket Map**: `map[uint64][]uint32` stores user IDs in each cell.
-   **Reverse Index**: `map[uint32]uint64` tracks which cell each user belongs to for $O(1)$ updates.
-   **Complexity**:
    *   Update: $O(1)$ average.
    *   Query: $O(CellsInRadius)$ - usually constant for fixed radii.

### Concurrency Model
-   Go uses a `sync.RWMutex` to allow multiple concurrent visibility queries while protecting updates.
-   `sync.Pool` is used for Protobuf message objects to minimize Garbage Collection (GC) overhead during high-frequency telemetry.

## License

MIT - See [LICENSE](LICENSE) for details.
