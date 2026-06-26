# Release v0.7.0 — Push blocker + manual instructions

## Status

- ✅ Tag `v0.7.0` created locally (commit `4ff7fb1`)
- ✅ All 6 cross-compiled binaries built (`dist/v0.7.0-radiant-*`)
- ✅ `SHA256SUMS` generated
- ✅ Tarball created: `/tmp/radiant-v0.7.0.tar.gz` (20MB)
- ⚠️ **Push to remote: blocked — no remote configured**

## The blocker

The repository has no `origin` remote configured:

```bash
$ git remote -v
(empty)
```

This is intentional or oversight — the repo lives at:
`/Users/henrique/Library/Mobile Documents/com~apple~CloudDocs/projects/radiant-harness-main`
(iCloud local storage). No `git@github.com:quant-risk/radiant-harness.git` (or similar) is set up.

## To push when ready

```bash
# Option A: Add GitHub remote (most likely path)
cd "/Users/henrique/Library/Mobile Documents/com~apple~CloudDocs/projects/radiant-harness-main"

# 1. Create the repo on GitHub first (https://github.com/quant-risk/radiant-harness)
# 2. Add the remote:
git remote add origin git@github.com:quant-risk/radiant-harness.git
# (or https://github.com/quant-risk/radiant-harness.git for HTTPS)

# 3. Push main + the v0.7.0 tag:
git push origin main
git push origin v0.7.0

# 4. (Optional) Push the release artifacts via gh CLI:
gh release create v0.7.0 \
    dist/v0.7.0-radiant-* \
    dist/SHA256SUMS \
    --title "v0.7.0 — Harness-Quant + niche skills + CLI commands" \
    --notes-file docs/RELEASE-v0.7.0.md
```

## Manual release alternative (no remote push)

If you want to publish the artifacts without setting up a Git remote,
the tarball + checksums are ready:

- **Tarball**: `/tmp/radiant-v0.7.0.tar.gz` (20MB, includes 6 binaries + SHA256SUMS)
- **Checksums**: `dist/SHA256SUMS`
- **Binaries individually**:
  - `dist/v0.7.0-radiant-linux-amd64`
  - `dist/v0.7.0-radiant-linux-arm64`
  - `dist/v0.7.0-radiant-darwin-amd64`
  - `dist/v0.7.0-radiant-darwin-arm64`
  - `dist/v0.7.0-radiant-windows-amd64.exe`
  - `dist/v0.7.0-radiant-windows-arm64.exe`

These can be uploaded to:
- GitHub Releases (after creating the repo)
- S3 / GCS / Azure Blob (cloud bucket)
- Internal artifact server (Artifactory, Nexus)

## Why this is documented, not faked

`git push origin` to a non-existent remote would fail. The right
behaviour is to surface the missing configuration, document the
commands to set it up, and ship the release artifacts so the user
can complete the publish step when ready.

## What's in v0.7.0

23 CLI commands + 53 bundled skills + 1 open schema + 1 strategic plan.
See `docs/validation-report-sprint-29.md` for full inventory.