# Release runbook

Cutting a release is gated on a human-signed tag. Everything up to that tag is
preparation; the signed tag is the publish decision, and only a maintainer makes
it. This runbook ends at that gate and resumes after the pipeline runs.

Releases follow [Semantic Versioning](https://semver.org). The examples use
`0.1.0`; substitute the version you are cutting.

## 0. Preconditions

- Release from the default branch, at the exact commit you intend to publish
  (merge the PR first; do not tag a feature branch).
- The signing identity is configured and its **public key is registered on
  GitHub** under the maintainer's account (SSH signing key, type "Signing Key"),
  so the tag shows as **Verified**. This repo signs with SSH:

  ```sh
  git config user.signingkey      # must be set
  git config gpg.format           # ssh
  ```

  Registering the key is the maintainer's step; it cannot be done from CI.

## 1. Pre-flight checks

From a clean checkout of the release commit:

```sh
go build ./...
go vet ./...
go test ./...
test "$(grep -c require go.mod)" -eq 0   # zero third-party deps in core
go test ./internal/conformance/          # stance + zero-dep invariants
```

All must pass. If any fails, stop — do not tag.

## 2. Finalize the changelog

In [`CHANGELOG.md`](CHANGELOG.md), rename the `[Unreleased]` section to the
version with today's date, add a fresh empty `[Unreleased]` above it, and update
the compare links at the bottom:

```markdown
## [Unreleased]

## [0.1.0] - 2026-10-12
...

[Unreleased]: https://github.com/AGenNext/Agent-Chat/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/AGenNext/Agent-Chat/releases/tag/v0.1.0
```

Commit this (signed) to the default branch before tagging.

## 3. Confirm version consistency

The version appears in several files; confirm they all read the version being
cut (for `0.1.0` they already do):

- `deploy/helm/agennext-chat/Chart.yaml` — `version` and `appVersion`
- `deploy/helm/agennext-chat/values.yaml` and `values-prod.yaml` — `image.tag`
- `install/agennextd.rb` — `version` and the two download URLs
- `install/install.sh`, `scripts/build-release.sh`, `scripts/build-macos.sh` —
  default `VERSION`

```sh
grep -rn "0\.1\.0" deploy/helm install scripts | grep -v digest
```

## 4. THE GATE — sign and push the tag (maintainer only)

This is the publish decision. No history is rewritten; the tag only adds.

```sh
git tag -s v0.1.0 -m "agennextd 0.1.0"
git tag -v v0.1.0          # confirm the signature verifies
git push origin v0.1.0
```

Pushing the tag triggers the release workflow. (A tag, once pushed, is public;
do not delete and re-push a published tag — cut a new patch version instead.)

A manual fallback exists if a tag is not desired: run the **release** workflow
via `workflow_dispatch` with the version as input. It publishes the image but
does not attach binaries to a GitHub release (that step is tag-gated).

## 5. What the pipeline does (automatic, on the tag)

The `release` workflow (`.github/workflows/release.yaml`):

- **image** — builds the container, pushes it to GHCR by digest with SBOM and
  provenance attestations, and signs the digest with cosign (keyless, via the
  workflow's OIDC identity — the platform signs its own artifacts, not
  contracts). The digest is printed to the run summary.
- **binaries** — cross-compiles `agennextd` for linux/macOS/windows
  (amd64 + arm64 where applicable), archives and checksums them, and creates the
  GitHub release `v0.1.0` with generated notes, attaching the archives and
  `checksums.txt`.

## 6. Post-release backfill

1. **Homebrew formula.** From the release's `checksums.txt`, copy the two
   `darwin` sha256 values into `install/agennextd.rb`, replacing
   `REPLACE_WITH_ARM64_SHA256` and `REPLACE_WITH_AMD64_SHA256`. Commit (signed).
2. **Immutable image pin.** Copy the image digest from the run summary into
   `deploy/helm/agennext-chat/values-prod.yaml` as
   `image.tag: "0.1.0@sha256:<digest>"` for a pinned production deploy.

## 7. Verify

```sh
# Image is signed and the digest resolves.
cosign verify ghcr.io/agennext/agent-chat:0.1.0 \
  --certificate-identity-regexp '.*' --certificate-oidc-issuer-regexp '.*'

# Binaries match their checksums.
sha256sum -c checksums.txt

# The installed binary runs one turn end-to-end.
agennextd -demo
```

The tag shows as **Verified** on GitHub, the release lists the binaries, and the
image is signed. If any check fails, the release is not done — investigate
before announcing it.

---

Outstanding founder decisions that affect a release but are not part of this
runbook: the licence (the Homebrew formula currently assumes `Apache-2.0` with a
TODO), the legal entity, and jurisdiction. Resolve these before a public 1.0.
