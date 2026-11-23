# Handoff

Last updated: 2026-11-23 by Backstage docs-standard rollout · branch `main`

## State

Latest tag is **v0.3.0** (2026-10-20); there are three annotated tags (v0.1.0, v0.2.0, v0.3.0) and no
GitHub Releases. The repo holds the four lab store apps (vitrine, api, cofre, sonda) that the mentoring labs
run; the labs themselves live in the private `StaffOps-Mentoring/mentoria-roadmap`. Since v0.3.0 the images
were renamed to `ghcr.io/staffops-mentoring/mentoria-apps/staffops-<app>` and a Backstage catalog and
TechDocs site were added; that work is not tagged yet (see `[Unreleased]` in the
[CHANGELOG](https://github.com/StaffOps-Mentoring/mentoria-apps/blob/main/CHANGELOG.md)). The last six `publicar` runs on `main` succeeded, the latest on
2026-11-18. All four Components are `experimental` in the catalog. No work is in flight in this repo.

## Done

- Backstage catalog, technology tags and TechDocs ([`b9ea64c`](https://github.com/StaffOps-Mentoring/mentoria-apps/commit/b9ea64c), [`e7b8a11`](https://github.com/StaffOps-Mentoring/mentoria-apps/commit/e7b8a11))
- Images published under the `staffops-` prefix; 0.2.0 and 0.3.0 copied to the new names by a one-off
  workflow that was then removed ([`c2eb0e9`](https://github.com/StaffOps-Mentoring/mentoria-apps/commit/c2eb0e9), [`28483d4`](https://github.com/StaffOps-Mentoring/mentoria-apps/commit/28483d4))
- 0.3.0: Dockerfiles without the `# syntax` line, so builds work offline ([`a8afcb9`](https://github.com/StaffOps-Mentoring/mentoria-apps/commit/a8afcb9))
- 0.2.0: vitrine requires `SEGREDO_SESSAO` and `API_URL` when `AMBIENTE=producao` ([`96393d8`](https://github.com/StaffOps-Mentoring/mentoria-apps/commit/96393d8))
- 0.1.0: vitrine, api, cofre and sonda, multi-arch images on GHCR ([`645092d`](https://github.com/StaffOps-Mentoring/mentoria-apps/commit/645092d))

## In progress

- No open pull requests and no open issues. Check with `gh pr list -R StaffOps-Mentoring/mentoria-apps`.

## Next steps

1. Tag the next release when the renamed images and the catalog changes are worth a version (suggested,
   inferred from the untagged `[Unreleased]` entries).
2. Translate the README into English, the org docs language (suggested, inferred from the docs standard).

## Decisions

No ADRs in this repo. The design rule is in the README: lab defects come from configuration or the admin
port, never from broken images, so one image serves many labs and a scenario can be drawn at random.

## Risks and open questions

- Labs pin images by digest. The mentoring repo's kubernetes handoff records the lab bases on **v0.2.0**;
  whether they moved to v0.3.0 or to the `staffops-` names is not stated here (open question).
- GHCR packages in the `StaffOps-Mentoring` org are created private. Today the four `staffops-<app>`
  packages are public and the four old names (`mentoria-apps/<app>`) are private; a new image name must be
  made public before a lab can pull it.
- `sonda` has no tests; only its image build runs in CI.

## How to verify

```bash
gh run list -R StaffOps-Mentoring/mentoria-apps -L 5
git tag -n
docker run --rm -v "$PWD/apps/vitrine:/src" -w /src golang:1.26-alpine go test ./...
docker build --build-arg VERSAO=dev -t vitrine:dev apps/vitrine
docker run --rm -p 8080:8080 -e LOJA_NOME=demo vitrine:dev   # then: curl localhost:8080/healthz
```
