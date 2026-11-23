# AGENTS.md

Container images that the StaffOps mentoring labs (docker, pipelines, kubernetes) package, publish and run:
vitrine, api, cofre and sonda, one directory each under `apps/`.

## Branches

| Branch | Use |
|---|---|
| `main` | The only long-lived branch. Every push publishes `:main` and `:sha-<short>` images |
| short branches | Work goes through a pull request into `main`; a PR only tests and builds, it never pushes |

A tag `vX.Y.Z` publishes `:X.Y.Z`. Do not push to `main` or create a tag unless the owner asked for it.

## Commands

Go and Docker run in containers, never on the host.

```bash
# Format, vet and test one app (what CI runs for vitrine, api and cofre)
docker run --rm -v "$PWD/apps/vitrine:/src" -w /src golang:1.26-alpine \
  sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test ./...'

# Build and run an image locally
docker build --build-arg VERSAO=dev -t vitrine:dev apps/vitrine
docker run --rm -p 8080:8080 -e LOJA_NOME=demo vitrine:dev
```

CI: [`.github/workflows/publicar.yml`](.github/workflows/publicar.yml) (test matrix, then a multi-arch image per app).

## Rules

- New documentation, commit messages and PR text are in English. The existing README is in Portuguese.
- Conventional Commits (`feat(vitrine): ...`, `fix(api): ...`, `docs: ...`).
- Update `CHANGELOG.md` `[Unreleased]` and the affected doc in the same commit as the change.
- Never commit secrets. `SEGREDO_SESSAO`, `TOKEN` and the `DB_URL` password are runtime inputs; the apps never
  print their values.
- Lab defects come from configuration (env vars, flags, manifests) or the admin port, never from deliberately
  broken images: one image serves many labs.

## Pitfalls

| Pitfall | What to do |
|---|---|
| `# syntax=docker/dockerfile:1` makes BuildKit download its frontend, so offline builds fail | Keep it out of every Dockerfile (fixed in 0.3.0) |
| Labs pin images by digest, not tag | A new tag does not reach a lab until the lab's digest is updated in `StaffOps-Mentoring/mentoria-roadmap` |
| `sonda` has no Go code | It is not in the CI test matrix, only in the image matrix |

## Docs map

| Need | Read |
|---|---|
| Apps, env vars, routes, exit codes | [README.md](README.md) |
| Current state of the work | [docs/handoff.md](docs/handoff.md) |
| Release history | [CHANGELOG.md](CHANGELOG.md) |
