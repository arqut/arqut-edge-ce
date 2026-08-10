# Contributing to Arqut Edge CE

Thanks for your interest in improving Arqut. This document covers how to get a
change accepted.

## Developer Certificate of Origin (DCO)

We use the [Developer Certificate of Origin](https://developercertificate.org/)
rather than a Contributor License Agreement. There is no paperwork to sign — you
certify the origin of your contribution by signing off each commit.

Every commit must carry a `Signed-off-by` trailer matching the author:

```
Signed-off-by: Jane Doe <jane@example.com>
```

Git adds it for you with `-s`:

```bash
git commit -s -m "fix: restart proxy on port change"
```

Forgot to sign off? Amend the last commit:

```bash
git commit --amend -s --no-edit
```

For a whole branch:

```bash
git rebase --signoff main
```

CI enforces this on every pull request. By signing off you assert the following.

<details>
<summary>Developer Certificate of Origin 1.1 (full text)</summary>

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same license (unless I am permitted to submit
    under a different license), as indicated in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

</details>

## Licensing of contributions

Arqut Edge CE is licensed under the Apache License 2.0. Contributions are
accepted under the same license, including the patent grant in section 3.

## Development setup

Prerequisites: Go 1.24 or newer, and Node.js with npm for the management UI.

```bash
go mod download
make install-ui     # install UI dependencies
make build          # builds the UI, then the Go binary
make test
```

`make dev-ui` runs the UI with live reload; `.air.toml` configures live reload
for the Go side. See `make help` for the full target list.

Note that `make build` runs `build-ui` before `build-go` for a reason: the
compiled SPA in `ui/dist/spa` is embedded into the binary (`ui/embed.go`).
Building only the Go side will embed a stale UI.

## Before opening a pull request

- `make test` passes.
- `go vet ./...` is clean.
- UI changes pass `npm run lint` in `ui/`.
- New code has tests.
- Commits are signed off (see above).
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
  (`fix:`, `feat:`, `docs:`, `chore:`), matching the existing history.

## Adding dependencies

New dependencies — Go modules and npm packages alike — must be under a
permissive license (MIT, BSD, Apache-2.0, ISC, or MPL-2.0). GPL, LGPL, and AGPL
dependencies cannot be accepted; they are incompatible with how this project is
distributed.

This applies to UI dependencies as much as Go ones. The production npm tree is
bundled into `ui/dist/spa` and embedded in the released binary, so those
packages are redistributed and must be attributed. When you add a dependency,
note it in your pull request so `NOTICE` and `THIRD_PARTY_LICENSES` can be
regenerated. Adding a `devDependency` used only at build time does not require
attribution.

## Reporting security issues

Do not open a public issue for a security vulnerability. See
[SECURITY.md](SECURITY.md) for how to report privately.
