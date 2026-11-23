# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html),
and commit messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

## [Unreleased]

### Changed

- Images are published as `ghcr.io/staffops-mentoring/mentoria-apps/staffops-<app>` (previously
  `ghcr.io/karlipegomes/mentoria-apps/<app>`). Versions 0.2.0 and 0.3.0 were copied to the new names by a
  one-off workflow, which was then removed.

### Added

- Backstage `catalog-info.yaml` (one System, one Component per image) and a TechDocs site (`mkdocs.yml`).

## [0.3.0] - 2026-10-20

### Fixed

- Dockerfiles no longer start with `# syntax=docker/dockerfile:1`, so images build without internet access
  (the line made BuildKit download its frontend).

## [0.2.0] - 2026-10-20

### Added

- vitrine: with `AMBIENTE=producao`, `SEGREDO_SESSAO` and `API_URL` are required; a missing one exits with
  code 3 and the last log line names the variable.

## [0.1.0] - 2026-10-20

### Added

- The lab store apps: vitrine (HTTP front), api (Postgres backend), cofre (on-disk key-value store) and sonda
  (curl, dig and nc toolbox), published to GHCR for `linux/amd64` and `linux/arm64`.

[Unreleased]: https://github.com/StaffOps-Mentoring/mentoria-apps/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/StaffOps-Mentoring/mentoria-apps/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/StaffOps-Mentoring/mentoria-apps/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/StaffOps-Mentoring/mentoria-apps/tree/v0.1.0
