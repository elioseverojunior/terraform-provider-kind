// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

//go:build generate

// Package tools pins the versions of the code generators this project uses, so
// contributors do not need them installed globally.
//
// Licence headers are NOT generated: this project is dual-licensed under
// MIT OR Apache-2.0, and hashicorp/copywrite only accepts a single SPDX
// identifier, not an expression. REUSE.toml carries the declaration instead.
package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

// Format the Terraform snippets that get embedded into the generated docs.
//go:generate terraform fmt -recursive ../examples/

// Regenerate docs/ from the provider schema, templates/ and examples/.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir .. -provider-name kind
