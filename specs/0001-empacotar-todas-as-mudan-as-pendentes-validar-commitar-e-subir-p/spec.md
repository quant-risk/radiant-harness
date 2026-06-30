# Spec — empacotar, validar, commitar e subir mudanças

Status: closed by the v3.7.5 release and subsequent cleanup commits.

## Goal

Empacotar mudanças pendentes, validar, criar commits e subir para o GitHub
como versão atualizada.

## Acceptance criteria

- Working tree reviewed before staging.
- Validation gates pass before push.
- Commits are pushed to `main`.
- Public GitHub release exists for the drop-in package.

## Non-goals

- Rewriting unrelated history.
- Bundling personal user cases into the harness.

## Verification

- `git status --short`
- `go test ./...`
- `make test-dropin`
- GitHub release `v3.7.5`
