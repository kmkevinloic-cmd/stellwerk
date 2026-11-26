# Versioning and releases

## Policy

Stellwerk follows [Semantic Versioning](https://semver.org/). One version
covers the whole repository; the edge library, the control plane and the
simulator are always released together.

While the version is `0.x`:

* **minor** (`0.2.0`): new features, or any breaking change to an API or to
  the heartbeat protocol;
* **patch** (`0.1.1`): bug fixes and documentation only.

The heartbeat protocol has its own version (`"v": 1`). A change that old nodes
or an old control plane cannot understand requires a new protocol version,
and the receiver keeps accepting the previous one for at least one minor
release.

## Single source of truth

The release version lives in [`VERSION`](../VERSION). `scripts/version.sh`
keeps every component in sync with it, and CI fails if they drift apart:

```bash
scripts/version.sh check         # all components agree with VERSION?
scripts/version.sh bump 0.2.0    # update VERSION and every component
```

## Making a release

1. Move the notes under `## [Unreleased]` in [CHANGELOG.md](../CHANGELOG.md)
   into a new `## [X.Y.Z]` section and add its compare link at the bottom.
2. `scripts/version.sh bump X.Y.Z`, then `make test`.
3. Commit: `git commit -am "release: vX.Y.Z"`.
4. Tag and push: `git tag -a vX.Y.Z -m "Stellwerk vX.Y.Z" && git push origin main vX.Y.Z`.

Pushing the tag runs the release workflow, which checks that the tag matches
`VERSION`, runs the tests, builds `stellwerkd` for Linux (amd64, arm64) and
creates a GitHub release with the changelog section as release notes.

## Commit messages

Commits start with the component they touch: `edge:`, `control-plane:`,
`sim:`, `docs:`, `build:` or `release:`.
