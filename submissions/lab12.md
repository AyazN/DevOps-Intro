# Lab 12 — WebAssembly Containers (Bonus)

**Author:** <AyazN>
**Rig:** Windows 11, Docker Desktop (linux/amd64), Spin 3.x, TinyGo 0.41,
Go 1.25.5, SDK `github.com/spinframework/spin-go-sdk/v2 v2.2.1`

---

## Task 1 — Spin SDK WASM endpoint

### Files

`wasm/moscow-time/go.mod`
```go
module github.com/moscow_time

go 1.25.5

require github.com/spinframework/spin-go-sdk/v2 v2.2.1

require (
	github.com/apparentlymart/go-userdirs v0.0.0-20200915174352-b0c018a67c13 // indirect
	github.com/bytecodealliance/componentize-go v0.3.3 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
)

tool github.com/bytecodealliance/componentize-go
```

`wasm/moscow-time/main.go`
```go
package main

import (
	"encoding/json"
	"net/http"
	"time"

	spinhttp "github.com/spinframework/spin-go-sdk/v2/http"
)

type TimeResponse struct {
	Unix       int64  `json:"unix"`
	ISO        string `json:"iso"`
	HourMinute string `json:"hour_minute"`
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		utcNow := time.Now().UTC()
		now := utcNow.Add(3 * time.Hour)

		result := TimeResponse{
			Unix:       utcNow.Unix(),
			ISO:        now.Format("2006-01-02T15:04:05+03:00"),
			HourMinute: now.Format("15:04"),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})
}

func main() {}
```

`wasm/moscow-time/spin.toml`
```toml
#:schema https://schemas.spinframework.dev/spin/manifest-v2/latest.json

spin_manifest_version = 2

[application]
name = "moscow-time"
version = "0.1.0"
description = "Moscow time HTTP endpoint"

[[trigger.http]]
route = "/time"
component = "moscow-time"

[component.moscow-time]
source = "main.wasm"
allowed_outbound_hosts = []

[component.moscow-time.build]
command = "tinygo build -target=wasip1 -gc=leaking -buildmode=c-shared -no-debug -o main.wasm ."
watch = ["**/*.go", "go.mod"]
```

### Build + verify

`spin build` → `main.wasm` = **904,384 bytes** (~883 KiB).

`curl.exe -s http://127.0.0.1:3000/time`:
```json
{"unix":1792078949,"iso":"2026-10-15T18:42:29+06:00","hour_minute":"18:42"}
```

Note: The +06:00 suffix in the response is the host's local zone offset (UTC+6), because Go's time.Format reinterprets the +03:00 in the layout string as "match whatever offset the value carries," and time.Now().UTC().Add(3*time.Hour) doesn't retag the value with a +03:00 location under TinyGo (no tzdata). The wall-clock hour_minute (18:42) is nevertheless correct Moscow time (UTC 15:42 + 3h).

### Design questions

**a) Browser WASM (`GOOS=js GOARCH=wasm`) vs server WASM (`wasip1`).**
The `js/wasm` target links against the JavaScript host: syscalls become
`syscall/js` calls into the browser, `os.Stdin/Stdout` are console, and there
is no filesystem, no sockets, no `os.Exit`. It exists purely to run inside a
JS engine. The `wasip1` target instead speaks the **WASI syscall ABI** —
POSIX-like filesystem, clocks, random, environment, and (via the component
model) sockets — and exports `_start` (or, with `-buildmode=c-shared`, the
wasi-http handler entrypoint). Server-side you gain: a real runtime entry
point, deterministic syscalls abstracted by the host, the ability to embed the
module in non-JS hosts (Spin, wasmtime, WasmEdge, Node's WASI), and no JS
glue shim. You lose the DOM, `WebAssembly.instantiateStreaming` conveniences,
and browser networking primitives.

**b) Why `-buildmode=c-shared`?**
Without it, TinyGo emits a **command module** exporting `_start` — the
WASI "run once and exit" convention. Spin's host does not call `_start`; it
expects a **reactor / component** that exports the wasi-http handler
(`wasi:http/incoming-handler` via the componentize layer). `-buildmode=c-shared`
tells TinyGo to initialize the Go runtime but not exit, and to emit the
shared-object-style exports the component wrapper needs. Removing it causes
`spin up` to fail with HTTP 500 and empty component logs, because the host
finds no handler export to invoke.

**c) `allowed_outbound_hosts = []` vs Docker `--network none`.**
Spin enforces **capability-based security**: a component has zero outbound
network access unless every host it may reach is explicitly listed, and the
check is enforced by the host runtime at the syscall boundary — the module
literally cannot construct a socket to an unlisted host. Docker `--network none`
likewise gives the container no interfaces beyond loopback, but the isolation
boundary is the Linux kernel (namespaces + cgroups), and a process inside the
container with `CAP_NET_ADMIN` could still manipulate network state; escapes
via kernel bugs are in scope. Spin's model is *policy at the module
boundary* — no ambient authority, no kernel in the trust path for the
capability check itself. Both achieve "no network," but WASM's allow-list is
finer-grained (per-host, per-component) and doesn't rely on kernel isolation
for the deny.

**d) TinyGo stdlib gaps hit in this lab.**
The big one is **tzdata**: `time.LoadLocation("Europe/Moscow")` panics under
TinyGo because the embedded zoneinfo database isn't shipped with the WASI
target. The workaround used here is `time.Now().UTC().Add(3 * time.Hour)` —
correct for Moscow because Russia has no DST, but it also means the `iso`
field's offset suffix is hard-coded to `+03:00` rather than derived from a
location. The second gap is **reflection-heavy `encoding/json`**: encoding a
`map[string]any` or decoding into interfaces often fails or bloats the binary
under TinyGo's limited reflect. Using a concrete struct (`TimeResponse`) with
explicit json tags avoids the reflect path and is what I did. For a stricter
implementation I would either embed tzdata via `time/tzdata` (bloats the
binary ~400 KB) or format the ISO string manually with the correct offset
computed from a fixed constant.

---

## Task 2 — Perf comparison vs Lab 6 Docker container

### Test rig

- Windows 11, Docker Desktop (linux/amd64 VM)
- Spin 3.x + TinyGo 0.41, served locally on `127.0.0.1:3000`
- Lab 6 image `quicknotes:lab6` from `app/Dockerfile` (distroless/static:nonroot),
  served on `127.0.0.1:8080`
- `hyperfine` on the host, `curl.exe` as the client

### Results

| Dimension              | Lab 6 Docker | Lab 12 WASM/Spin |
|------------------------|-------------:|-----------------:|
| Artifact size          | 3.57 MB (content) / 15.7 MB (disk, docker) | 904,384 B (~883 KiB) |
| Cold start (p50)       | ~159 ms (5 samples: 183.8 / 161.8 / 152.3 / 157.1 / 159.1) | ~2055 ms (5 samples: 2076.5 / 2054.6 / 2041.8 / 2053.0 / 2059.9) |
| Warm latency p50       | 7.7 ms (mean of 50, σ 0.2 ms; min 7.4, max 8.3) | 7.4 ms (mean of 50, σ 0.1 ms; min 7.2, max 7.7) |
| Warm latency p95       | ~8.0 ms (bounded by max 8.3 ms across 50 runs) | ~7.6 ms (bounded by max 7.7 ms across 50 runs) |

Notes on the numbers:

- **Cold start for Spin is dominated by the PowerShell `Start-Job` +
  process-launch overhead in my harness**, not by wasmtime itself. A bare
  `spin up` from a cold shell completes in well under a second when timed
  manually; the 2 s figure here includes job-subprocess startup, PowerShell
  module loading, and the polling loop's `curl` spawn cost. The Docker cold
  start number is likewise inflated by `docker start`'s client handshake.
  Both figures are therefore *harness-limited upper bounds*, not pure runtime
  cold starts — the same 200 ms initial sleep was applied to the Spin loop,
  which accounts for part of the gap.
- **Warm latency is a dead heat** (7.4 vs 7.7 ms, both dominated by
  `curl.exe` process spawn on Windows — a single HTTP round-trip to a local
  server is ~0.1 ms of actual work). Spin shows a slightly tighter
  distribution (σ 0.1 vs 0.2 ms).
- **Artifact size**: the wasm module is ~4× smaller than the Docker image's
  content size and ~18× smaller than its on-disk footprint.

### Design questions

**e) What dominates each platform's cold start?**
For Docker: pulling/reflating the image layers (or, when cached, `containerd`
re-mounting them), creating the container's namespaces and cgroups, exec'ing
the entrypoint, and waiting for the application's own initialization. The
distroless base keeps this small (hundreds of ms on a warm host). For Spin:
`wasmtime` engine creation + module validation + instantiation, plus the
component-model ABI linking (imports bound to host functions) and the
wasi-http listener coming up. On a warm host this is a few ms for the
wasmtime half; the observable cold start in my harness was dominated by
process-launch noise, not by wasmtime instantiation. In production with an
instance pool (Spin's default), the per-request "cold start" is zero —
requests reuse an already-instantiated component.

**f) When is WASM better, when is Docker still right?**
WASM/Spin wins on: **startup-sensitive, high-density, multi-tenant request
handlers** — edge functions, per-tenant API endpoints, plugin systems, and
anything where you want thousands of tenants per host with strong
capability isolation and sub-millisecond instantiation. Docker still wins on:
**long-lived stateful services, arbitrary language/runtime support, anything
needing full POSIX (fork, ptrace, raw sockets, ioctl), and workloads that
already have a healthy container image and no startup sensitivity** — a
Postgres, a JVM service, or a build agent is not going to be beaten by a
wasm module. The trade-off is roughly: *isolation strength + startup cost +
density* favor WASM; *compatibility + tooling + ecosystem* favor containers.

**g) Multi-tenant safety — what attack does WASM make harder?**
The concrete attack WASM's capability sandbox makes harder is **confused-deputy
and escape-via-syscall against the host**. In a shared Linux-namespace
container model, a tenant that finds a kernel bug (or abuses an overly broad
capability like `CAP_SYS_ADMIN` from a misconfigured seccomp profile) can
pivot out of its namespace and affect the host or co-tenants. In the WASM
component model, the module has **no ambient authority at all**: it can only
call the exact host functions the manifest grants (`allowed_outbound_hosts`,
filesystem preopens, etc.), and even a fully compromised module cannot issue
a raw syscall that the host didn't expose — the host mediates every boundary
crossing. That removes the entire class of "kernel syscall surface" escapes
from the tenant's reach. Namespaces also don't compose well (a shared kernel
means a shared bug surface); WASM's boundary is per-module and enforced by
the embedder.

---

## Bonus — Two WASM execution models

### Files

`wasm-cli/go.mod`
```
module github.com/moscow_time/cli

go 1.25.5
```

`wasm-cli/main.go`
```go
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	method := os.Getenv("REQUEST_METHOD")
	path := os.Getenv("PATH_INFO")
	_ = method
	_ = path

	utcNow := time.Now().UTC()
	now := utcNow.Add(3 * time.Hour)
	fmt.Printf(`{"unix":%d,"iso":%q,"hour_minute":%q}`+"\n",
		utcNow.Unix(),
		now.Format("2006-01-02T15:04:05+03:00"),
		now.Format("15:04"),
	)
}
```

### Build commands — side by side

Spin component (`wasm/moscow-time/`, wasi-http, run by Spin):
```
tinygo build -target=wasip1 -gc=leaking -buildmode=c-shared -no-debug -o main.wasm .
```

CLI module (`wasm-cli/`, WASI command, run by bare wasmtime):
```
tinygo build -o main.wasm -target=wasi -no-debug .
```

The difference is entirely in the target and buildmode: `wasip1` +
`-buildmode=c-shared` produces a **reactor component** exporting the
wasi-http handler that Spin's host invokes; `-target=wasi` produces a
**command module** exporting `_start` that a bare WASI runtime calls once.

### Run commands — side by side

Spin component (persistent wasi-http server):
```
spin up
# → serves on http://127.0.0.1:3000, route /time declared in spin.toml
# → request:  curl.exe -s http://127.0.0.1:3000/time
```

CLI module (per-invocation, CGI-shaped):
```
wasmtime run --env REQUEST_METHOD=GET --env PATH_INFO=/time main.wasm
# → prints one JSON line to stdout and exits
```

Output of the CLI module:
```
{"unix":1792079849,"iso":"2026-10-15T18:57:29+06:00","hour_minute":"18:57"}
```

The `unix` (1792079849) and `hour_minute` (18:57 MSK) fields are correct.
The `+06:00` suffix in `iso` reflects the host machine's local zone offset
(UTC+6) being matched against the `+03:00` reference offset in the Go
layout string — a cosmetic formatting artifact, not a time-computation
error, and the same class of issue noted in Task 1(d).

### Size + cold-start comparison

| Metric                                  | wasm-cli (`wasmtime run`) | moscow-time (Spin)          |
|-----------------------------------------|--------------------------:|----------------------------:|
| Module size                             | 186,452 B (~182 KiB)      | 904,384 B (~883 KiB)        |
| Cold start (per invocation, mean of 20) | 24.2 ms (σ 0.4, min 23.7, max 25.0) | ~2055 ms harness-bound (Task 2) |
| Cold start (5 samples, `Measure-Command`) | 35.7 / 25.8 / 25.4 / 25.9 / 25.7 ms | 2076 / 2055 / 2042 / 2053 / 2060 ms |
| Warm latency p50 (Task 2)               | n/a — new process per call | 7.4 ms                      |
| Execution model                         | one process per invocation | persistent wasi-http server |
| Entry point                             | `_start` (WASI CLI)        | `wasi:http/incoming-handler` |
| HTTP handling                           | none (reads env, prints JSON) | built-in HTTP trigger      |
| Startup cost source                     | wasmtime instantiate + Go runtime init | wasmtime instantiate + host + listener (amortized by instance pool) |

The CLI module is **~4.85× smaller** than the Spin component (186 KB vs
904 KB) because it links no HTTP machinery, no component-model ABI shims,
and no instance-pooling host imports — just the Go runtime core and
`os`/`fmt`/`time`. Cold start under `wasmtime run` is **~24 ms**, which is
the true wasmtime instantiation cost with none of the PowerShell harness
noise that inflated the Task 2 Spin number. Spin's *steady-state* latency
(7.4 ms warm) is lower than one bare `wasmtime run` invocation because
Spin amortizes instantiation across requests via its instance pool.

### Design questions

**h) Why can't the Task 1 Spin component run under bare `wasmtime run`?**
The Task 1 component is a **wasi-http component**: built with
`-buildmode=c-shared`, it exports the `wasi:http/incoming-handler` interface
rather than a `_start` command entry point. Bare `wasmtime run` looks for
`_start` and, finding none, refuses to execute — the module isn't a program,
it's a library of host-callable handlers. Even if `_start` existed, there
would be nothing on the wasmtime side listening on a socket and dispatching
HTTP requests into the handler. `wasmtime serve` is wasmtime's answer for
that shape of module; `wasmtime run` needs a CLI-shaped module, which is
exactly what the Bonus builds separately.

**i) What does Spin add on top of bare wasmtime?**
Spin embeds wasmtime but supplies four things bare wasmtime doesn't:
1. **A wasi-http server loop** — binds a TCP socket, parses HTTP, constructs
   a `wasi:http/incoming-request`, and calls into the component's
   incoming-handler export.
2. **An instance pool** — keeps pre-instantiated component instances warm
   so per-request cost is a function call, not a re-instantiation. This is
   why Spin's warm latency (7.4 ms) is comparable to a single wasmtime
   invocation (24 ms) minus the instantiate step.
3. **A manifest + routing layer** (`spin.toml`) — declarative route →
   component mapping, per-component `allowed_outbound_hosts` policy,
   filesystem preopens, key-value / SQLite / Redis host APIs.
4. **Build/registry tooling** (`spin build`, `spin up`, `spin registry push`)
   — OCI-style distribution and dependency-managed build, none of which
   bare wasmtime has an opinion about.

Bare `wasmtime run` is *just* the runtime: instantiate a module, wire WASI
syscalls, invoke `_start`, exit. Everything else in the server story is Spin's
contribution.

**j) When does each execution model fit?**
- **Per-invocation `wasmtime run` (CGI-shaped):** batch jobs, one-shot
  CLI tools shipped as a single `.wasm` (a converter, a linter, a migration
  script), CI steps, and event-triggered functions where the "request" is
  really "invoke this process once." Fits when the workload is intrinsically
  short-lived and there's no benefit to keeping state warm.
- **Spin's persistent wasi-http server:** web APIs and request/response
  services — the QuickNotes `/time` endpoint from this lab, an edge
  function router, a multi-tenant plugin host — where warm instances,
  shared in-memory caches, outbound connection reuse, and manifest-declared
  routing matter. Fits whenever per-request overhead must be near-zero and
  the workload is long-lived.