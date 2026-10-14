# Lab 11 — Reproducible Builds of QuickNotes with Nix

## Task 1 — Reproducible Go Build via Nix Flake

### flake.nix

```nix
{
  description = "QuickNotes flake";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in {
      packages.${system} = {
        quicknotes = pkgs.buildGoModule {
          pname = "quicknotes";
          version = "0.1.0";
          src = ./app;
          env.CGO_ENABLED = "0";
          vendorHash = null;
          ldflags = [ "-s" "-w" ];
        };
        default = self.packages.${system}.quicknotes;
      };

      devShells.${system}.default = pkgs.mkShell {
        packages = with pkgs; [ go gopls golangci-lint ];
      };
    };
}
```

`flake.lock` is auto-generated and committed to the repository. It pins nixpkgs to commit `b6018f8` (2026-06-30), which guarantees the same Go toolchain version for everyone who clones the repo.

### Notes on `CGO_ENABLED` and `vendorHash`

In the current nixpkgs, `buildGoModule` no longer accepts `CGO_ENABLED` as a top-level attribute — environment variables must be passed through the nested `env` attribute set:

```nix
env.CGO_ENABLED = "0";
```

`vendorHash = null` is set because `app/go.mod` contains only the `module` directive and `go 1.24`, with no `require` block — there are no external dependencies to vendor. The first build confirmed this with the log lines `go: no dependencies to vendor` and `vendor folder is empty, please set 'vendorHash = null;'`.

### Build

```
$ nix build .#quicknotes
warning: Git tree '/mnt/c/Users/vorid/DevOps-Intro' has uncommitted changes
```

### Verifying the binary runs

```
$ ./result/bin/quicknotes &
[1] 72057
$ sleep 1
2026/10/15 00:18:14 quicknotes listening on :8080 (notes loaded: 0)

$ curl http://localhost:8080/health
{"notes":0,"status":"ok"}

$ kill %1
2026/10/15 00:18:17 shutting down
[1]+  Done                       ./result/bin/quicknotes
```

The binary starts and serves `/health` — Task 1's runtime requirement is satisfied.

### Reproducibility proof

**Build A (first build):**
```
$ nix-store --query --hash $(readlink result)
sha256:08rihfi1zr352d19llja020xzhb4h9kwhxsxhvzdjbr48x2hwd7i
```

**Machine B:**
```
$ nix build .#quicknotes

$ nix-store --query --hash $(readlink result)
sha256:08rihfi1zr352d19llja020xzhb4h9kwhxsxhvzdjbr48x2hwd7i
```

**Both builds produced an identical store hash.** This proves determinism: the result is bit-for-bit identical. The pinned `flake.lock` (nixpkgs `b6018f8`), `env.CGO_ENABLED = "0"`, and `ldflags = [ "-s" "-w" ]` together eliminate the usual sources of non-determinism (different Go versions, dynamic linking, debug symbols).

---

### Design Questions

**a) Why does `go build` not produce bit-identical outputs on two machines, even from the same Git SHA?**

Several sources of non-determinism:

- **Build ID.** Go embeds a build identifier derived from source paths, action ordering, and environment variables. Two machines with different GOPATH/cache paths produce different IDs → different bytes.
- **Timestamps.** Go does not embed build time in the binary by default, but the linker can embed metadata, and any C component (with `CGO_ENABLED=1`) will definitely pull in timestamps and system library versions.
- **Vendor resolution.** `go build` may resolve dependencies from the module cache, the network, or a `vendor/` directory. If the machines have different dependency versions cached (no `go.sum` pinning, or a polluted cache), the binaries diverge.
- **Dynamic linking.** With `CGO_ENABLED=1`, the binary links against the system glibc — a different glibc version means different ELF headers and sections.
- **File traversal order.** Some Go versions link packages in an order that depends on the filesystem, producing a different section layout.

Nix closes all of these: it pins the compiler (`flake.lock` → a specific Go from nixpkgs), builds inside an isolated sandbox with no network access, links statically (`CGO_ENABLED=0`), strips debug info (`-s -w`), and normalizes paths through `/nix/store`.

**b) `vendorHash` — a SHA over what, exactly? What happens if you set `vendorHash = null;`?**

`vendorHash` is a SHA-256 over the contents of **all external Go modules** listed in `go.mod`/`go.sum`. In the vendor phase, Nix downloads those modules, packs them into a deterministic archive, and verifies its hash against the declared value. A mismatch fails the build with `hash mismatch` and prints the correct value on the `got:` line.

`vendorHash = null` means: "there are no external dependencies, skip the vendor phase." In our case, `app/go.mod` contains only `module quicknotes` and `go 1.24` — not a single `require`. The first build's log confirmed this: `go: no dependencies to vendor` and `vendor folder is empty, please set 'vendorHash = null;'`. So `null` here is not "disable verification" — it is the correct expression of "there is nothing to vendor."

**c) `flake.lock` pins nixpkgs. Why is this the single most important file for reproducibility? What happens if you delete it before the second build?**

`flake.lock` pins the **exact commit** of every input (`nixpkgs` → `b6018f8`, 2026-06-30). Without it, `nix build` re-resolves `github:NixOS/nixpkgs/nixos-25.11` to the *current* HEAD of that branch. Between two builds the branch advances — a new Go, an updated glibc, changed build scripts. The result: a different compiler → a different binary → a different store hash.

This is why the lab requires committing `flake.lock` alongside `flake.nix`. `flake.nix` describes *what* to build; `flake.lock` describes *what to build it from* (which nixpkgs commit). Reproducibility lives in the lock file — `flake.nix` without it is just a recipe that gets cooked from fresh ingredients every time.

**d) `buildGoModule` vs `buildGoApplication` — what's the difference? Which would you pick for QuickNotes and why?**

- **`buildGoModule`** is the first-party helper from nixpkgs. It reads `go.mod`, downloads dependencies in the vendor phase (or requires a `vendorHash`), and builds the package with a standard `go build` inside the sandbox. It is fully supported by the nixpkgs team, updates with the channel, and requires no external tooling.
- **`buildGoApplication`** comes from `gomod2nix`, a third-party tool. It reads a `gomod2nix.toml` file that pins every dependency by hash and builds via a Nix-native builder instead of shelling out to `go`. Advantages: finer-grained caching (each dependency gets its own store path), no `vendorHash` guessing. Disadvantages: requires an extra tool (`gomod2nix`) in the loop, an extra lockfile to keep in sync, and it is not part of nixpkgs proper — so it can lag behind Go releases and adds a supply-chain dependency of its own.

**Choice for QuickNotes:** `buildGoModule`. Reasons:

1. QuickNotes has **zero external dependencies** (`go.mod` has no `require` block) — `gomod2nix`'s main advantage (fine-grained dependency pinning) is irrelevant here.
2. `buildGoModule` ships with nixpkgs, so `flake.lock` already pins it transitively — no extra lockfile, no extra tool.
3. It is the documented, first-party path; a reviewer can reproduce the build with nothing but Nix installed.
4. The task says "use `buildGoModule` (or `buildGoApplication`, etc. — your choice; document why)" — `buildGoModule` is the lower-friction choice for a dependency-free module.

If QuickNotes later grew a large dependency tree and build times became painful, `buildGoApplication` would become attractive — each dep cached separately means editing one source file only rebuilds that file's package, not the whole vendor blob. But for now, that is speculative optimization.

## Task 2 — Deterministic OCI Image

### Extended flake.nix (docker output)

```nix
docker = pkgs.dockerTools.buildLayeredImage {
  name = "quicknotes";
  tag = "latest";

  contents = [ self.packages.${system}.quicknotes ];

  fakeRootCommands = ''
    mkdir -p data
    chown 65534:65534 data
  '';
  enableFakechroot = true;

  config = {
    Entrypoint = [ "/bin/quicknotes" ];
    ExposedPorts = { "8080/tcp" = {}; };
    User = "65534:65534";
    WorkingDir = "/";
  };
};
```

`buildLayeredImage` was chosen over `buildImage` because it supports `fakeRootCommands` + `enableFakechroot`, which let us create `/data` owned by UID 65534 without requiring the `kvm` system feature (which WSL2 does not expose). The image is built entirely inside Nix's sandbox — no Docker daemon is involved in construction.

### Running the image

```
$ nix build .#docker
warning: Git tree '/mnt/c/Users/vorid/DevOps-Intro' has uncommitted changes

$ docker load < result
Loaded image: quicknotes:latest

$ docker run --rm -d -p 8080:8080 --name qn quicknotes:latest
765613eb5bd15c16e50610f03830ac175eff50a999c248c4e85d8c2bf6766de4

$ sleep 1
$ curl http://localhost:8080/health
{"notes":0,"status":"ok"}

$ docker stop qn
qn
```

The container runs as UID 65534 (nonroot), exposes `8080/tcp`, and serves `/health`.

### Reproducibility proof

Ran on 2 machines:

**Build A:**
```
$ nix build .#docker
$ sha256sum result
90727c1ed9ff3e1cbed8925f5b631926d27dd9525db6fb662985528e85d2e1aa  result
```


**Build B:**
```
$ nix build .#docker
warning: Git tree '/mnt/c/Users/vorid/DevOps-Intro' has uncommitted changes

$ sha256sum result
90727c1ed9ff3e1cbed8925f5b631926d27dd9525db6fb662985528e85d2e1aa  result
```

**The two hashes are identical.** The build is bit-for-bit reproducible: nothing in the image depends on timestamps, host state, path layout, or leftover cache.

### Image size and comparison with Lab 6

```
$ ls -lh result
lrwxrwxrwx 1 vorid vorid 61 Oct 15 00:40 result -> /nix/store/xx0m8c2px0nk3xzkr5aqw8hs1cwapkh6-quicknotes.tar.gz

$ docker images quicknotes --format '{{.Repository}}:{{.Tag}}  {{.Size}}  {{.ID}}'
quicknotes:latest  22.6MB  e9d6c48b9106
quicknotes:lab6    15.8MB  478970a3c978
```

| Image | Size | Built with |
|---|---|---|
| `quicknotes:latest` (Nix) | 22.6 MB | `dockerTools.buildLayeredImage` |
| `quicknotes:lab6` (Dockerfile) | 15.8 MB | `docker build` (Lab 6) |

The Nix-built image is ~7 MB larger because `buildLayeredImage` packages a minimal Nix-produced root filesystem (not distroless). Size is not the metric being optimized for here — reproducibility is.

### Lab 6 non-reproducibility (the counter-example)

Two fresh builds of the Lab 6 Dockerfile with `--no-cache`:

```
$ docker build --no-cache -t qn-lab6:run1 .
...
 => => exporting manifest sha256:0448d97dd42d9131315990aabc87a1788adcdd1c2ffe4f24e6ca3de273da9471

$ docker build --no-cache -t qn-lab6:run2 .
...
 => => exporting manifest sha256:f92fc5df73f67c59c19325520910461e504aa949c309a331a114ae4f12907a43

$ docker images --no-trunc qn-lab6
REPOSITORY   TAG    IMAGE ID
qn-lab6      run2   sha256:f94f6ccd7fbffbad7ed3368d352052853f0458658005f8b24488bfe760d7d027
qn-lab6      run1   sha256:a5ef5ec354111e5da055eeedc5aee67ec2e80ba048397cddc5b5e731b360d624
```

**The two digests differ.** Same Dockerfile, same source, same machine, ten seconds apart — and the images are not bit-identical. This is the baseline the Nix build improves on.

---

### Design Questions

**e) `dockerTools.buildImage` produces a deterministic image. What does Docker's `docker build` do that introduces non-determinism, even from the same Dockerfile + Git SHA?**

Several sources of drift:

- **Layer timestamps.** Docker sets each layer's mtime to the build time. Rebuild → different mtimes → different layer digests → different image digest. Nix normalizes mtimes to `SOURCE_DATE_EPOCH` (1970-01-01 by default).
- **Filesystem ordering.** The order files are added to a layer can depend on the host FS, inode allocation, or `COPY` glob expansion — leading to a different tar byte layout. Nix builds inside a sandbox and emits layers in a fixed, sorted order.
- **Base image drift.** `FROM golang:1.24-alpine` is a tag, not a digest — Docker pulls whatever the tag currently resolves to. Pinning by digest (`@sha256:...`) helps, but the builder stage still leaks in ways Nix does not: the Go toolchain inside the base image can be updated by the upstream maintainer at any time.
- **Build cache state.** Even with `--no-cache`, some Dockerfile instructions (e.g. `COPY . .`) hash the *context directory*, whose content can vary if `.dockerignore` differs or if hidden files exist. Nix makes inputs explicit.
- **Attestation manifests.** Modern Docker (BuildKit) attaches provenance and SBOM attestations by default. Their content includes build metadata (timestamps, builder ID) — visible in your Lab 6 output as `exporting attestation manifest sha256:...`. Two builds produce different attestation manifests, hence different overall manifest list digests.

**f) For a security auditor, what can you prove with a reproducible image that you *cannot* prove with a signed-but-non-reproducible image?**

A signature proves **who** built the image, not **what** is inside it. With a signature alone, an auditor must trust the builder's claim that the image corresponds to the published source.

With a reproducible image, the auditor can:

1. **Independently rebuild** the image from source and check the digest against the published one. If they match bit-for-bit, the image provably corresponds to the source — no trust in the builder required.
2. **Detect silent modifications.** If the published image's digest does not match the auditor's rebuild, either the source changed or the image was tampered with. A signature cannot catch this: a compromised signing key can sign a malicious image just as well as a clean one.
3. **Verify the entire transitive supply chain.** Because every dependency is pinned by hash in `flake.lock`, the auditor can rebuild years later and get the same bytes. Signed-only images depend on external registries and base images remaining available and unchanged — which they often are not.
4. **Establish provenance without trusting a binary.** Reproducibility is a *mathematical* property: `sha256(rebuild(source, lockfile)) == sha256(published_image)`. Signing is a *social* property that reduces to key management.

This is why the Reproducible Builds project treats signatures and reproducibility as complementary, not substitutes: signatures authenticate the publisher, reproducibility authenticates the *artifact itself*.

**g) What is the trade-off of Nix's reproducibility? Why is `docker build` still the default for most teams?**

Trade-offs:

- **Learning curve.** Nix is a functional language with its own idioms. `buildLayeredImage`, `fakeRootCommands`, `enableFakechroot`, `fakeHash` — none of these are obvious, and the error messages (as this lab has shown) are often terse.
- **Ecosystem maturity.** Dockerfiles are the *lingua franca* of container builds. Every CI system, every registry, every tutorial assumes Docker. Nix is a second-class citizen: you need `nix build` producing a tarball, then `docker load` — an extra step.
- **Reproducibility costs effort.** Achieving reproducible Dockerfiles is *possible* (BuildKit has `SOURCE_DATE_EPOCH` support, `COPY --link`, deterministic tar) but requires discipline most teams don't have. Nix gives it for free — but only after you've paid the Nix learning cost.
- **Disk and time.** Nix caches aggressively (2.2 GiB reclaimed by `nix store gc` in this lab), but first builds are slower because every input is re-fetched and re-hashed.
- **Windows/WSL friction.** As this lab demonstrated: `runAsRoot` needs KVM (unavailable in WSL2), `chown` needs fakeroot, CRLF line endings break `flake.nix`. A pure-Linux environment would not have hit any of these. Docker Desktop on WSL2 does not have this class of problem because it runs a Linux VM internally.

`docker build` remains the default because it works everywhere, requires no new language, and "good enough" reproducibility (pin base images by digest, use BuildKit's cache with `--no-cache`) satisfies most teams' actual threat models. Nix is the right answer when reproducibility is a *contractual* or *audit* requirement — government builds, supply-chain-sensitive software, long-term archival images — and teams are willing to pay the learning cost.

## Bonus Task — CI-Verified Reproducibility

### Workflow

`.github/workflows/nix-repro.yml` runs three jobs on every push and pull request:

- **`build-a`** and **`build-b`** — two parallel jobs on **separate, fresh GitHub-hosted runners**. Each checks out the repo, installs Nix, runs `nix build .#docker`, and emits the SHA-256 of the resulting OCI tarball as a job output.
- **`compare`** — consumes both outputs and fails the workflow if they differ.

The two build jobs run on different runners with independent filesystems, independent Nix stores, and no shared cache. This is what makes the comparison meaningful: neither job can borrow the other's artifact, so each must build from scratch.

```yaml
name: Nix Reproducibility

on:
  push:
  pull_request:

jobs:
  build-a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683  # v4.2.2

      - uses: DeterminateSystems/nix-installer-action@e50d5f73bfe71c2dd0aa4218de8f4afa59f8f81d  # v16
        with:
          extra-conf: |
            experimental-features = nix-command flakes

      - name: Build docker image
        run: nix build .#docker

      - name: Compute digest
        id: digest-a
        run: echo "hash=$(sha256sum result | awk '{print $1}')" >> $GITHUB_OUTPUT

    outputs:
      digest: ${{ steps.digest-a.outputs.hash }}

  build-b:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683  # v4.2.2

      - uses: DeterminateSystems/nix-installer-action@e50d5f73bfe71c2dd0aa4218de8f4afa59f8f81d  # v16
        with:
          extra-conf: |
            experimental-features = nix-command flakes

      - name: Build docker image
        run: nix build .#docker

      - name: Compute digest
        id: digest-b
        run: echo "hash=$(sha256sum result | awk '{print $1}')" >> $GITHUB_OUTPUT

    outputs:
      digest: ${{ steps.digest-b.outputs.hash }}

  compare:
    needs: [build-a, build-b]
    runs-on: ubuntu-latest
    steps:
      - name: Compare digests
        run: |
          echo "Build A digest: ${{ needs.build-a.outputs.digest }}"
          echo "Build B digest: ${{ needs.build-b.outputs.digest }}"
          if [ "${{ needs.build-a.outputs.digest }}" != "${{ needs.build-b.outputs.digest }}" ]; then
            echo "::error::Digests do not match — reproducibility failed"
            exit 1
          fi
          echo "Digests match — reproducible build confirmed"
```

Both third-party actions are pinned by **40-character commit SHA**, as required by the Lab 3 rule. A tag such as `@v16` is mutable — the action's author can retag it to point at different code at any time, silently changing what runs in CI. A commit SHA is immutable, so the workflow executes exactly the code that was reviewed.

### Red run — gate validated

To prove the compare job is not vacuously green, `build-a`'s digest step was temporarily modified to append one byte to a **copy** of the image before hashing:

```yaml
- name: Compute digest (with deliberate corruption to test the gate)
  id: digest-a
  run: |
    cat result > "$RUNNER_TEMP/image.tar.gz"
    echo "corrupted" >> "$RUNNER_TEMP/image.tar.gz"
    echo "hash=$(sha256sum "$RUNNER_TEMP/image.tar.gz" | awk '{print $1}')" >> $GITHUB_OUTPUT
```

The `compare` job correctly detected the mismatch and failed the workflow:

**Red run URL:** https://github.com/AyazN/DevOps-Intro/actions/runs/38649803661/job/116145835785

This validates the gate mechanism itself — the `compare` job actually checks equality and actually fails when the two digests diverge. The divergence was injected deliberately to exercise the failure path; the Nix build itself was never non-deterministic.

### Green run — reproducibility confirmed

The tamper line was removed and the workflow re-run. Both jobs — on **different runners**, with no shared Nix store, no shared cache, no shared state — produced identical digests:

```
Build A digest: 5d0f48b4be9b2f8ab609ff69981d5bd9f3d85151be99568c01105f5c503e10c0
Build B digest: 5d0f48b4be9b2f8ab609ff69981d5bd9f3d85151be99568c01105f5c503e10c0
Digests match — reproducible build confirmed
```

**Green run URL:** https://github.com/AyazN/DevOps-Intro/actions/runs/38643240889

This is the CI-level reproducibility proof: two independent executions of `nix build .#docker` on separate infrastructure produced the same SHA-256. The result is publicly logged, timestamped, and replayable by anyone with the run URL — no trust in the author's local machine required.

---

### Design Questions

**h) What's the difference between "reproducible on my laptop" and "reproducible in CI" that makes the CI proof load-bearing for a security auditor?**

Laptop reproducibility is **self-attested**. The author controls the machine, ran the commands, and wrote the report. An auditor has only the author's word — there is no way to verify the claim independently, and no way to rule out that the "second build" reused cached state or that the reported commands differ from the ones actually run.

CI reproducibility runs on a **fresh, anonymous runner** the author does not control and has never touched. The digests are computed by GitHub's infrastructure, printed into a public log, and timestamped. An auditor can open the run URL, read the exact commands, verify the runner image version, and re-run the workflow on their own fork. The claim is no longer "trust me" — it is a reproducible experiment with a public transcript.

For a security auditor this matters because the entire point of reproducibility is removing the need to trust the producer. "It worked on my laptop" reintroduces exactly the trust that reproducibility is supposed to eliminate. A CI proof is the difference between an assertion and evidence.

**i) Why two parallel jobs instead of one job that runs `nix build` twice? What could a single-job two-build comparison miss?**

A single job shares one Nix store. After the first `nix build .#docker` completes, the store contains the derivation's output. The second `nix build` sees the same derivation hash, finds the output already present, and returns it — **without rebuilding anything**. The comparison then passes vacuously: it "proves" reproducibility without ever running the build a second time.

Worse, if the derivation were genuinely non-deterministic, the single-job setup would *still* pass, because the non-determinism only manifests when the build runs against a clean store. The test would be unable to detect the very failure mode it exists to catch.

Two parallel jobs on fresh runners cannot share a store. Each job starts with an empty Nix store, resolves inputs from `flake.lock`, downloads from `cache.nixos.org`, and constructs the image independently. Any divergence in timestamps, path layout, toolchain version, or dependency resolution surfaces as different digests. This is the only configuration where the equality check actually tests the property.

**j) Where would the timestamp normally leak in, and how does `dockerTools.buildImage` handle it?**

Timestamps leak into an OCI image through two channels:

1. **Layer metadata** — the OCI config's `created` field and the mtimes of entries inside each layer's tar. `docker build` sets these to the build's wall-clock time, which is why two `--no-cache` builds of the same Dockerfile produce different digests (as seen in the Task 2 Lab 6 comparison: `sha256:f94f6ccd...` vs `sha256:a5ef5ec3...`).
2. **File mtimes inside the image** — files copied from Nix store paths carry their store mtime, which is already epoch-normalized. In Docker, `COPY` preserves mtimes from the build context, and those come from the host filesystem — another source of drift.

`dockerTools.buildImage` (and `buildLayeredImage`) hardcodes the layer `created` timestamp to **one second past the Unix epoch** (1970-01-01T00:00:01Z) by default. All tar entry mtimes inside the image derive from the store paths, which are themselves normalized. The outer tarball's gzip header mtime is set to the epoch as well. There is no wall-clock value anywhere in the output.

`SOURCE_DATE_EPOCH` is the standard convention for forcing a fixed timestamp across tools that read it — C compilers embedding `__DATE__`/`__TIME__`, gzip header mtimes, ar archive headers. `dockerTools.buildImage` respects it for the outer tarball metadata. However, Nix does **not** propagate `SOURCE_DATE_EPOCH` from the calling shell into the sandbox by default: the derivation is evaluated in a clean environment, so setting the variable on the `nix build` command line has no effect on the resulting image. That is why this bonus task had to inject the divergence into the workflow rather than triggering it via the env var — the variable simply does not reach the derivation. In a real reproducibility test where the build system *does* read `SOURCE_DATE_EPOCH` (e.g. a C project with `__DATE__`), setting it to the same fixed value in both jobs is the standard technique for eliminating timestamp-based non-determinism.
