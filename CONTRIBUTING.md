# Contributing

runhug-cli is released under the [MIT license](LICENSE). By contributing
you agree that your contribution is distributed under that license.
Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

Community health files follow
[adamsiwiec1/foss-template](https://github.com/adamsiwiec1/foss-template)
(the adam-foss baseline used by OpenHat Security and The FreeTech Company).

## Development

Requires [Go](https://go.dev/dl/) 1.22 or later.

```bash
go test ./...
go vet ./...
go build -o bin/runhug-cli ./cmd/runhug-cli
```

Docs (optional — Node 22, same floor as the FOSS template):

```bash
npm ci
npm run docs:dev
npm run docs:build
```

Do not commit secrets, credentials, `.env` files, registry files that contain
API keys, or personal data. `RUNPOD_API_KEY` and `HF_TOKEN` stay in the
environment.

## Documentation

`docs/guide/` is task-oriented. `docs/reference/` is lasting fact.
GitHub-conventional files stay at the repository root. Update the docs in the
same pull request as the behavior they describe.

## Changelog

Every user-visible change needs a [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
bullet under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md) in the same PR.
Before a release, move Unreleased into a dated `## [x.y.z] - YYYY-MM-DD`
section. Use [Semantic Versioning](https://semver.org/).

## Pull requests

- One concern per PR when you can.
- Describe the change and any security or privacy impact.
- Add or update tests for behavior changes.
- Update docs and the changelog when behavior changes.
- Preserve attribution and verify license compatibility for reused code.

Security vulnerabilities must follow [SECURITY.md](SECURITY.md), not public
issues.

See [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/)
and [Using pull requests](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/about-pull-requests).
