<!--
SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>

SPDX-License-Identifier: MIT OR Apache-2.0
-->

# Contributing

Thanks for helping out. Bug reports, documentation fixes and new features are
all welcome.

## Getting set up

You need Go (the version in [`go.mod`](go.mod)) and a running Docker daemon.
Terraform is only needed for acceptance tests and for `terraform fmt`.

```shell
git clone https://github.com/elioseverojunior/terraform-provider-kind.git
cd terraform-provider-kind
make build
```

`golangci-lint` is required for `make lint`. Install it from
[golangci-lint.run](https://golangci-lint.run/welcome/install/).

## The loop

```shell
make test        # unit tests, no Docker needed, ~2s
make lint        # golangci-lint, must report 0 issues
make generate    # regenerate docs/ after any schema change
make testacc     # acceptance tests: creates real clusters, needs Docker, slow
```

Before opening a pull request, run all four. `make check` runs everything except
the acceptance tests.

## Testing

Unit tests never touch Docker. They inject a fake through the `clusterManager`
interface, so the whole CRUD surface runs in milliseconds — see
[ARCHITECTURE.md](ARCHITECTURE.md#the-kind-boundary).

```shell
make test                     # everything
make test-coverage            # writes coverage.out and prints the total
go test ./internal/provider/ -run TestBuildNodeConfig -v
```

New code needs tests. The bar is: **every branch a practitioner can reach must
be exercised.** Coverage currently sits above 95%; the uncovered remainder is
`main()`'s serve call and one defensive error branch. If your change drops the
number, add tests rather than lowering the bar.

Acceptance tests are gated behind `TF_ACC`, the standard Terraform convention.
They create real clusters that take minutes and leave containers behind if
interrupted — clean up with `kind delete clusters --all`.

```shell
make testacc
go test ./internal/provider/ -run TestAccClusterResource_lifecycle -v -timeout 30m
```

### Writing tests

- Table-driven where the cases are genuinely parallel; separate named tests
  where each case needs its own explanation.
- `t.Parallel()` everywhere except tests calling `t.Setenv`, which forbids it.
- Say **why** in the test name and a comment when the behaviour is not obvious.
  `TestWaitForNodesReady_TimeoutNamesNotReadyNodes` explains itself; `TestWait3`
  does not.

## Documentation

`docs/` is generated. **Never edit it by hand** — run `make generate`.

The published Registry pages come from three sources:

| To change | Edit |
| --------- | ---- |
| An argument's description, type or allowed values | The `Schema` function in `internal/provider/` |
| The prose around the schema | `templates/*.md.tmpl` |
| A code sample | The `.tf` files in `examples/` |

Schema descriptions **are** the documentation, so write them for someone who has
never seen the source. Keep `examples/*.tf` free of licence headers: they are
embedded verbatim into the Registry code blocks and get copy-pasted into other
people's configurations.

CI fails if `make generate` produces a diff, so regenerate before pushing.

## Adding a kind capability

kind is reached exclusively through the `clusterManager` interface in
`internal/provider/cluster_manager.go`. To use a kind API the provider does not
call yet:

1. Add the method to `clusterManager`. The compile-time assertion in that file
   confirms `*cluster.Provider` still satisfies it.
2. Add it to `fakeClusterManager` in `internal/provider/fakes_test.go`.
3. Write the failing test, then the implementation.

## Validators

Schema validators mirror kind's own rules — see
`pkg/internal/apis/config/validate.go` in `sigs.k8s.io/kind`. Anything that
passes `terraform plan` must be acceptable to kind. When adding a validator,
copy kind's rule rather than inventing one, and update the schema description in
the same commit so the Registry page stays truthful.

## Licensing

The project is dual-licensed under **MIT OR Apache-2.0**. By contributing you
agree your work is released under both.

`REUSE.toml` is the single source of truth: its aggregate annotation covers every
file, so a new source file does not strictly need its own header. Adding one
anyway keeps the tree consistent with the existing files:

```go
// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0
```

Headers are **not** generated. `hashicorp/copywrite` was removed because it only
accepts a single SPDX identifier and cannot express a dual licence. Do not add
headers to `examples/**`: those files are embedded verbatim into the published
Registry code blocks.

## Commits and pull requests

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
feat(cluster): support the nftables kube-proxy mode
fix(cluster): report which nodes were not ready on timeout
docs: explain the two-stage apply for the kubernetes provider
```

In the pull request, describe what changed and why. If behaviour changed, say
how an existing configuration is affected — this project is consumed from the
Terraform Registry, so a breaking schema change affects everyone on the next
version bump.

## Releasing

Releases are cut by pushing a `v*` tag; GoReleaser builds the binaries and the
GitHub Actions workflow signs them with the project GPG key. Update
[CHANGELOG.md](CHANGELOG.md) in the same commit as the tag.

## Code of conduct

Participation is governed by the [Code of Conduct](.github/CODE_OF_CONDUCT.md).
