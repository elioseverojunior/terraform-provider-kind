<!--
SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>

SPDX-License-Identifier: MIT OR Apache-2.0
-->

# Architecture

How the provider is put together, and why. Read this before making a change
that spans more than one file.

## Layout

| Path | Contents |
| ---- | -------- |
| `main.go` | Plugin entry point. Holds `providerAddress`, the registry address Terraform resolves. |
| `internal/provider/provider.go` | Provider-level schema and `Configure`. Builds the kind client and hands it to every resource and data source. |
| `internal/provider/cluster_manager.go` | The `clusterManager` interface — the only seam between this provider and kind. |
| `internal/provider/cluster_resource.go` | `kind_cluster`: schema, CRUD, config translation, readiness polling. |
| `internal/provider/cluster_resource_model.go` | Terraform-side structs. One struct per schema block. |
| `internal/provider/cluster_data_source.go` | `kind_clusters`. |
| `templates/` | `tfplugindocs` templates. **Edit these, never `docs/`.** |
| `examples/` | Terraform snippets. Embedded verbatim into the published docs. |
| `tools/` | Separate module pinning the doc/licence generators. |

## The kind boundary

The provider does not shell out to the `kind` binary. It imports
`sigs.k8s.io/kind` and drives `cluster.Provider` directly, which is why kind is
a normal Go dependency and why upgrading it is a code change rather than a
runtime concern.

Everything the provider needs from kind is declared as a four-method interface
in `cluster_manager.go`:

```go
type clusterManager interface {
	Create(name string, options ...cluster.CreateOption) error
	Delete(name, explicitKubeconfigPath string) error
	List() ([]string, error)
	KubeConfig(name string, internal bool) (string, error)
}
```

`*cluster.Provider` satisfies it, and a compile-time assertion in the same file
proves it, so a signature change upstream breaks the build rather than
production. Resources depend on the interface, never the concrete type — that is
what makes the CRUD paths testable in milliseconds with no Docker daemon. When
you need a new kind capability, add the method here first.

## Translating Terraform config into kind config

`buildClusterConfig`, `buildNetworkingConfig` and `buildNodeConfig` convert the
Terraform model into kind's `v1alpha4.Cluster`. They are pure functions: no I/O,
no receiver state. Keep them that way; they carry the densest test coverage in
the repo precisely because they are cheap to exercise.

Two conventions run through them:

- **Null and empty mean "unset".** A null attribute, an empty string, or an
  empty collection is omitted from the kind config so kind applies its own
  default. Writing a zero value through would override that default.
- **Null elements inside collections are skipped**, not forwarded as empty
  strings, which kind would reject at parse time.

## Validation mirrors kind

Schema validators reproduce kind's own rules rather than inventing new ones. The
cluster name regex in `cluster_resource.go` is copied from `validNameRE` in
kind's `pkg/internal/apis/config/validate.go`, and the enum sets come from the
same file.

The point is that anything passing `terraform plan` must be acceptable to kind.
If you relax or tighten a validator, check that file first — and if kind's rules
have moved, update both the validator and the schema description together, since
the description is what practitioners read on the Registry.

## Immutability

kind cannot reconfigure a running cluster: no adding nodes, no changing
networking. Every argument that feeds the cluster config therefore carries
`RequiresReplace`. `Update` exists only to refresh computed attributes when a
non-replacing argument (`wait_for_ready`, `wait_for_nodes_ready`) changes; it
never mutates a cluster.

## Readiness

After creation the provider optionally waits for every node to report `Ready`.
`waitForNodesReady` takes a `kubernetes.Interface` and a poll interval as
parameters so it can be driven by a fake clientset in tests; `waitForAllNodesReady`
is the production wrapper that builds a real client from the kubeconfig.

List errors and empty node lists are treated as "not ready yet" rather than
failures, because the API server is unreachable for the first moments after
creation. The timeout message names the nodes still pending, which is the only
diagnostic an operator gets.

## Documentation pipeline

`docs/` is **generated**. `make generate` runs `tfplugindocs`, which combines:

- the resource and provider schemas (descriptions, types, validators),
- the `.tf` files under `examples/`, embedded verbatim,
- the templates under `templates/`.

A schema description is documentation. Changing one changes the published
Registry page, so write it for a practitioner who has never read the source.
Never hand-edit `docs/` — the next generate run overwrites it.

## Testing strategy

| Layer | What it covers | Cost |
| ----- | -------------- | ---- |
| Pure builders | Config translation, null handling | microseconds |
| Schema tests | Validators, attribute wiring | microseconds |
| CRUD with fakes | Every code path through Create/Read/Update/Delete | milliseconds |
| Readiness with a fake clientset | Polling, retries, timeout messages | milliseconds |
| Acceptance (`TF_ACC=1`) | Real clusters, real Docker | minutes |

Only the acceptance tier needs Docker, and it is skipped by default. See
[CONTRIBUTING.md](CONTRIBUTING.md) for how to run each tier.
