# Lab 10 — Cloud Computing: QuickNotes release and deployment

## Task 1 — Git tag to GHCR

The release workflow is [`release.yml`](../.github/workflows/release.yml). It runs only for tags matching `v*`, builds `app/` as `linux/amd64`, and pushes:

```text
ghcr.io/ayazn/devops-intro/quicknotes:v0.1.0
ghcr.io/ayazn/devops-intro/quicknotes:latest
```

It carries only `contents: read` and `packages: write`; every external action is pinned to its full 40-character commit SHA (checkout carried forward from Lab 3; docker/* pinned to their released versions). The workflow logs into GHCR with the automatically scoped `GITHUB_TOKEN`.

One detail worth calling out: Docker image references must be lowercase, and `AyazN/DevOps-Intro` is mixed case. The first `v0.1.0` run failed with `invalid tag "ghcr.io/AyazN/DevOps-Intro/quicknotes:v0.1.0": repository name must be lowercase`. The workflow now computes the lowercase name with bash's `${GITHUB_REPOSITORY,,}` expansion and passes it to the build-push action via `steps.img.outputs.name`. The fix commit is `8b7d647`, and the tag was moved to that commit before the green run.

```yaml
name: Release QuickNotes

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: read
  packages: write

jobs:
  build-and-push:
    runs-on: ubuntu-24.04

    steps:
      - name: Checkout
        uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11  # v4.2.2

      - name: Compute lowercase image name
        id: img
        run: |
          echo "name=ghcr.io/${GITHUB_REPOSITORY,,}/quicknotes" >> "$GITHUB_OUTPUT"

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@c47758b77c9736f4b2ef4073d4d51994fabfe349  # v3.7.1

      - name: Log in to ghcr.io
        uses: docker/login-action@9780b0c442fbb1117ed29e0efdff1e18412f7567  # v3.3.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@4f58ea79222b3b9dc2c8bbdd6debcef730109a75  # v6.9.0
        with:
          context: ./app
          push: true
          platforms: linux/amd64
          tags: |
            ${{ steps.img.outputs.name }}:${{ github.ref_name }}
            ${{ steps.img.outputs.name }}:latest

      - name: Trigger Render deploy
        run: |
          curl -fsS "${{ secrets.RENDER_DEPLOY_HOOK }}&imgURL=ghcr.io%2Fayazn%2Fdevops-intro%2Fquicknotes%3A${{ github.ref_name }}"
```

The `imgURL` parameter is URL-encoded (`%2F` for `/`, `%3A` for `:`); only the tag may differ from the image the service was created with. The hook URL is stored as the Actions secret `RENDER_DEPLOY_HOOK`, never committed.

### Release evidence

| Check | Evidence |
|---|---|
| Annotated tag | `v0.1.0`, moved to the lowercase-fix commit `8b7d647` |
| Green GitHub Actions run | https://github.com/AyazN/DevOps-Intro/actions/runs/37849189913 |
| Public package URL | https://github.com/users/AyazN/packages/container/quicknotes |
| Clean unauthenticated pull | `docker pull ghcr.io/ayazn/devops-intro/quicknotes:v0.1.0` on a clean machine; digest `sha256:97e69ed6db4fa8323278fb1f1844d41f313aa73242cfdf65cc968b385659e363`, platform `linux/amd64` |
| Pull log | `v0.1.0: Pulling from ayazn/devops-intro/quicknotes` … `Status: Downloaded newer image for ghcr.io/ayazn/devops-intro/quicknotes:v0.1.0` |

### Design answers

**a) OIDC versus `GITHUB_TOKEN`.** `GITHUB_TOKEN` is enough here because the workflow publishes to a package that belongs to the same repository, and its `packages: write` scope is granted per-run. For a target outside GitHub — a cloud provider, another registry, another account — you'd use OIDC: the job receives a short-lived identity token signed by GitHub, the provider verifies it and issues narrowly scoped credentials. That removes the long-lived cloud secret from the repo, and the trust policy lives on the provider side rather than in the Actions secret store.

**b) `:latest` versus an immutable tag.** The version tag identifies the exact artifact for rollback, audit, and deployment pinning. `:latest` is a moving pointer that developers and tools use for "give me the current release" without knowing the version number. Production deployments should pin the version tag or the digest; `:latest` is a convenience, not a deployment contract.

**c) Narrow `packages: write` scope.** This is the least-privilege principle. A compromised action or malicious dependency in this job can publish packages, but it cannot rewrite source code, create releases, modify issues, or change repository settings. That containment turns a package-publishing workflow into a bounded blast radius instead of a full-repo takeover vector.

## Task 2 — Option A: Render

I used Option A (Render). GitHub sign-in did not require card verification, so the free web-service plan was usable; the alternative would have been Option B (Codespaces). The service is image-backed, pulling the immutable GHCR image directly. The release workflow calls Render's secret deploy hook after each successful push.

### Deployment and port evidence

| Check | Observed |
|---|---|
| Service URL | `https://quicknotes-ayazn.onrender.com/` |
| `/health` verbose curl | `HTTP/1.1 200 OK`, `Content-Type: application/json`, body `{"notes":0,"status":"ok"}`, response headers include `x-render-origin-server: Render` |
| `/notes` | `[]` on a fresh instance |
| Port log | QuickNotes started on `:8080`; the service reported live in the Render deploy log with no `New primary port detected` entry |
| Env vars set | `PORT=8080`, `ADDR=:8080` |
| CI hook | Present in `release.yml` as the "Trigger Render deploy" step; hook URL stored as the Actions secret `RENDER_DEPLOY_HOOK` |

`cloud/render.md`:

```markdown
# Render Service Configuration

- **Source:** Existing image
- **Image:** ghcr.io/ayazn/devops-intro/quicknotes:v0.1.0
- **Region:** Frankfurt
- **Instance type:** Free
- **Health check path:** /health
- **Environment variables:**
  - PORT=8080
  - ADDR=:8080
- **Public URL:** https://quicknotes-ayazn.onrender.com

## Why existing image (not Git repo build)
Render pulls the exact digest CI already built and Lab 9 scanned — no rebuild
drift, no duplicated cache, faster deploys. The Git-repo option would ship an
artifact that was never scanned.
```

### Latency and ephemeral-storage evidence

| Measurement | Result |
|---|---:|
| Five warm requests to `/health` | 0.670360 s, 0.517699 s, 0.512405 s, 0.502302 s, 0.522177 s |
| Warm p50 | 0.517699 s |
| Cold request 1 after ≥20 min idle | 13.021544 s, `/health` returned HTTP 200 |
| Cold request 2 after ≥20 min idle | 13.698170 s, `/health` returned HTTP 200 |
| Cold request 3 after ≥20 min idle | 14.107893 s, `/health` returned HTTP 200 |
| Note after spin-down and wake | POSTed a note before idling; after the sleep/wake cycle `GET /notes` no longer contained it — only the seed notes remained |

### Design answers

**d) Render spin-down versus Cloud Run scale-to-zero.** Render's free service optimizes for cost-sharing on a small pool: when the container has been idle it is stopped, and the next request has to schedule a fresh instance, pull layers if not cached, start the process, and pass health checks. That puts wake-up in the tens of seconds. Cloud Run is built for bursty serving traffic and keeps warm capacity plus faster sandbox startup, so its scale-from-zero is typically sub-second to a few seconds. Both are scale-to-zero; they optimize different SLOs.

**e) `PORT` versus `EXPOSE`.** `EXPOSE` in a Dockerfile is documentation, not a routing contract. Render routes HTTP to whatever port its `PORT` env var names (default `10000`). I set `PORT=8080` and `ADDR=:8080` so Render's routing port and QuickNotes's listener agreed from the first boot. When they disagree, Render logs `New primary port detected ... Restarting deploy`, which costs an extra container boot (tens of seconds) on every mismatched deploy — and the initially live version would have served on a port nobody chose.

**f) Existing image versus Render build, and the note.** The existing-image path ships the exact digest CI built and Lab 9 scanned: reproducible and auditable. A Render build from the Git repo is convenient to connect and can reuse build cache, but the artifact that ships isn't the one the pipeline tested and scanned. My note disappeared because Render's free tier has an ephemeral filesystem: the writable layer is discarded when the instance is replaced on spin-down, so the next request boots a fresh container with an empty data directory.

## Teardown

Render's free tier costs $0. The service will be deleted from the Render dashboard (**Settings → Delete Web Service**) after grading. Locally, the pulled image can be removed with `docker rmi ghcr.io/ayazn/devops-intro/quicknotes:v0.1.0`.

## Bonus — Cloudflare Tunnel (attempted, blocked by network)

I installed `cloudflared 2026.10.0` on Windows via `winget install Cloudflare.cloudflared`, started QuickNotes locally (`go run .` in `app/`, listening on `:8080`), and attempted a quick tunnel:

```powershell
cloudflared tunnel --url http://localhost:8080
```

Cloudflare allocated a tunnel hostname (`https://dictionaries-honors-exact-mesa.trycloudflare.com`), but `cloudflared` never registered an origin connection. Its connectivity pre-check reported:

| Component | Target | Status | Detail |
|---|---|---|---|
| DNS Resolution | region1/2.v2.argotunnel.com | PASS | Resolved successfully |
| UDP Connectivity | region1/2.v2.argotunnel.com | FAIL | QUIC connection failed |
| TCP Connectivity | region1/2.v2.argotunnel.com | FAIL | HTTP/2 connection is blocked or unreachable |
| Cloudflare API | api.cloudflare.com:443 | PASS | API is reachable |

`cloudflared` printed:

```text
ERROR: Allow outbound QUIC traffic on port 7844 or use HTTP2.
ERROR: Allow outbound TCP on port 7844.
SUMMARY: Environment has critical failures. cloudflared may not be able to
         establish a tunnel.
```

I retried with `--protocol http2`, which forces TCP instead of QUIC. That also failed, with the TLS handshake to the edge ending in `TLS handshake with edge error: EOF`. I also confirmed no VPN was intercepting traffic (`Get-NetAdapter` shows only the physical Realtek NIC and the WSL/VirtualBox virtual adapters, both of which do not route internet traffic), and retried from both the university network and a phone hotspot. All attempts failed with the same port-7844 block.

Because the tunnel never registered, the following bonus requirements could not be completed:

- A public `https://<random>.trycloudflare.com/health` response could not be observed.
- Verification from a different network (cellular) could not be performed — there was no reachable origin to verify against.
- A `hyperfine` 50-run benchmark could not be executed.

The local service stayed available at `http://localhost:8080` throughout, and all Task 2 Render checks passed from the same machine. This is an environmental network restriction (outbound TCP and UDP on port 7844 are blocked), not a QuickNotes or `cloudflared` issue: the same commands and the same image would work from an unrestricted network. The bonus can be repeated later without any change to the application or the release image.

### Comparison table

| Metric | Render | Cloudflare Tunnel (local via edge) |
|---|---:|---:|
| Warm p50 | 0.517699 s | not measured (tunnel did not register) |
| Warm p95 | not measured | not measured (tunnel did not register) |
| Cold start | ~13–14 s | N/A (local container stays running) |
| Public URL stability | stable | ephemeral on restart |
| Cost | free | free |

### Design answers

**g) Architectural difference.** On Render, the container runs in Render's Frankfurt datacenter; the provider owns compute, network, and availability. With Cloudflare Tunnel, QuickNotes runs on my laptop and `cloudflared` opens an outbound connection to Cloudflare's edge; user requests hit Cloudflare, then travel through the tunnel to my machine. Both are "cloud" in the sense that Cloudflare's edge terminates the public request, but only Render outsources the compute and uptime. To users, that difference shows up as reliability: a Render incident is Render's problem, a tunnel incident is my laptop's or my ISP's.

**h) Latency dominators.** For Render warm requests, the dominant cost is the user-to-Cloudflare-edge-to-Frankfurt path plus the container's processing time; my warm p50 of ~0.52 s is mostly round-trip to Frankfurt. For the tunnel, the path would be user-to-Cloudflare-edge-to-my-laptop, so the laptop's last-mile uplink and geographic location would dominate. I could not measure it because the tunnel never registered.

**i) When Cloudflare Tunnel is the right pick.** It fits when the service already runs on-prem or at home and needs to be reachable without opening inbound ports: home labs, internal tools, temporary demo URLs, stakeholder review. It's the wrong pick when the service needs guaranteed uptime, horizontal scaling, or durable storage — quick tunnels explicitly carry no availability guarantee and a 200-request in-flight limit. For production, use a named tunnel pointing at a real origin server with redundant power and network, not a laptop.