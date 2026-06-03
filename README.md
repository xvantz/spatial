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

```mermaid
sequenceDiagram
    participant NP as Node-Plane (Master)
    participant NATS as NATS Broker
    participant GW as Go-Worker (Coprocessor)

    Note over NP: 1. Update positions (40ms tick)
    Note over NP: 2. Update Local XOR Hash
    
    NP->>NATS: Publish spatial.telemetry (TelemetryBatch + Hash)
    NATS->>GW: Forward Telemetry
    
    Note over GW: 3. BulkUpdate Spatial Grid
    Note over GW: 4. Sync Remote XOR Hash

    NP->>NATS: Request spatial.query.visibility (BatchQuery)
    NATS->>GW: Forward Query
    
    Note over GW: 5. O(1) Grid Proximity Search
    GW->>NATS: Reply VisibilityResponse (Neighbors + Worker Hash)
    NATS->>NP: Forward Response

    Note over NP: 6. Desync Detection (History Hash Check)
    Note over NP: 7. If Desync -> Full Sync next tick
```

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
To run tests:
```bash
go test ./...
```

#### Node-Plane (Master)
Navigate to the `node-plane` directory, install dependencies, and start the simulation:
```bash
cd node-plane
bun install
export NATS_URL=nats://localhost:4222
bun run dev
```

## Benchmarks

Performance measured on **AMD Ryzen 5 6600H** (Go 1.25.9, 6 cores):

### Proximity Queries (`GetInRadius`)

| Scenario | Players | Radius | Time per Op | Memory / Allocations |
|----------|---------|--------|-------------|----------------------|
| **Sparse** | 100 | 50m | **360 ns** | 0 B/op (0 allocs) |
| **Sparse** | 1,000 | 50m | **425 ns** | 0 B/op (0 allocs) |
| **Sparse** | 10,000 | 50m | **2.3 µs** | 5.6 KB/op (3 allocs) |
| **Sparse** | 1,000 | 150m | **3.4 µs** | 0 B/op (0 allocs) |
| **Sparse** | 1,000 | 500m | **78 µs** | 0 B/op (0 allocs) |
| **Clustered** | 1,000 | 50m | **1.26 µs** | 0 B/op (0 allocs) |

### Bulk Updates (`BulkUpdate`)

| Batch Size | Time per Op | Memory / Allocations |
|------------|-------------|----------------------|
| 100 | **4.8 µs** | 0 B/op (0 allocs) |
| 1,000 | **50 µs** | 0 B/op (0 allocs) |

### Player Removal (`RemovePlayer`)

| Players | Time per Op | Memory / Allocations |
|---------|-------------|----------------------|
| 1,000 | **175 µs** | 0 B/op (0 allocs) |

### Performance Progression

| Operation | Original (README v1) | Current | Improvement |
|-----------|---------------------|---------|-------------|
| Query (100, r=50) | 1.6 µs | **0.36 µs** | **4.4× faster** |
| Query (1,000, r=50) | 2.3 µs | **0.43 µs** | **5.4× faster** |
| Query (10,000, r=50) | 16.7 µs | **2.3 µs** | **7.2× faster** |
| Bulk Update (1,000) | 68.6 µs | **50 µs** | **1.4× faster** |

### Key Takeaways

1. **Near-Constant Scaling**: Increasing the player count from 100 to 1,000 (10x) results in only a **1.2x** increase in query time at r=50, proving the efficiency of the voxel grid ($O(1)$ bucket access with inline position storage).
2. **Zero-Allocation Path**: For all proximity queries up to 10,000 players at reasonable radii, the search generates **zero garbage** on the hot path — only the result buffer expand triggers allocations.
3. **Real-World Performance**: At 150m radius (the default visibility range), a query takes just **3.4 µs** — a single core can serve over **290,000 visibility queries per second**.
4. **High Throughput**: The coprocessor can process approximately **20,000 full-world updates per second** (1000-player batches), leaving a massive performance overhead for other game logic.
5. **Clustered Scenarios**: Even with all 1000 players packed into a 100×100 hotspot, queries complete in **1.26 µs**, confirming the grid's robustness for MMO crowded-area scenarios.

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
-   **Bucket Map**: `map[uint64][]cellEntry` stores user IDs with inline positions in each cell.
-   **Reverse Index**: `map[uint32]uint64` tracks which cell each user belongs to for $O(1)$ updates.
-   **Complexity**:
    *   Update: $O(1)$ average.
    *   Query: $O(CellsInRadius)$ - usually constant for fixed radii.

### Concurrency Model
-   Go uses a `sync.RWMutex` to allow multiple concurrent visibility queries while protecting updates.
-   `sync.Pool` is used for Protobuf message objects to minimize Garbage Collection (GC) overhead during high-frequency telemetry.

## License

MIT - See [LICENSE](LICENSE) for details.
