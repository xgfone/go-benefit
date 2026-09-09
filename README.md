# Benefit Redemption Framework for Go

[![GoDoc](https://pkg.go.dev/badge/github.com/xgfone/go-benefit)](https://pkg.go.dev/github.com/xgfone/go-benefit)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xgfone/go-benefit/main/LICENSE)
![Minimum Go Version](https://img.shields.io/github/go-mod/go-version/xgfone/go-benefit?label=Go%2B)
![Latest SemVer](https://img.shields.io/github/v/tag/xgfone/go-benefit?sort=semver)

`go-benefit` is a provider-neutral framework for inspecting, evaluating,
redeeming, and optionally reversing one benefit at a time. It supports coupons,
vouchers, electronic cards, virtual goods, service entitlements, and internal
benefits without imposing one transaction or product model on applications.

## Scope

Every driver implements the interface `Driver`; `Reverser` is an
optional capability. No result-query operation is currently defined. Future
result-query operations can be added as optional capabilities without expanding
the core `Driver` interface. The package provides driver registration and
binding, normalized results, constraints, money, and operation contracts; it
does not provide persistence, transport DTOs, or multi-benefit orchestration.

See the runnable [end-to-end example](example_test.go) for a complete driver
definition, configuration schema, validation and binding flow, custom operation
context, constraints, inspection, evaluation, redemption, and reversal. Public
types and methods are documented on [GoDoc](https://pkg.go.dev/github.com/xgfone/go-benefit).

Install the package with:

```sh
go get github.com/xgfone/go-benefit
```

## Driver lifecycle

A `DriverDefinition` describes one registrable driver type. Its `DriverType` is
a stable namespaced value composed from the provider and benefit kind, such as
`douyin.coupon` or `meituan.coupon`. Open machine identifiers contain at least
two dot-separated segments; each segment uses lower-case letters, digits,
underscores, or hyphens and must start with a letter or digit.

`DriverDefinition.Descriptor()` returns complete driver type metadata, including
`ConfigSchema` and optional operation capabilities. `DriverRegistry.Descriptors()`
returns a consistent, independent snapshot of these descriptors sorted by type,
so management applications can list drivers and their configuration forms in
one query. For user-facing lists, convert each entry with
`DriverDescriptor.Descriptor()` to obtain only `Type`, `Name`, and `Icon`.
`DriverRegistry.ConfigSchema(type)` remains available for individual schema queries.

Management applications use `DriverDescriptor.ConfigSchema` to render configuration
forms and call `ValidateConfig` before encrypting and persisting operator input. A
`DriverConfig` is either empty, when the schema permits it, or a UTF-8 JSON
object. Schemas use JSON Schema Draft 2020-12 and may include UI extensions such
as `x-secret`. `DriverDefinition.ValidateConfig` is authoritative and must agree
with `CompileConfig`; it is side-effect free and does not populate compilation
caches. Runtime binding does not call management validation again, so
`CompileConfig` independently validates and parses persisted configuration.

At runtime, `DriverRegistry.Bind` calls deterministic, local `CompileConfig`
and obtains a driver from the resulting `DriverFactory`. Definitions and proxies
may cache factories for equal configurations, and a factory may return the same
driver or different lightweight drivers across calls. The resulting driver has
provider credentials and fixed matching rules already bound, so operation calls
do not repeatedly receive sensitive configuration. Compiled factories and bound
drivers must be safe for concurrent use.

## Operation inputs and behavior

`BenefitReference` is an opaque reference such as a bearer coupon code or
provider record ID. Redeem and Reverse require a non-empty reference; other
operations may allow an empty reference when their driver contract permits it.
It is not a provider API credential and is excluded from default JSON
serialization. Applications should authenticate requests and resolve device,
merchant, or user facts before constructing driver requests.

`DriverInput` carries application-defined, in-process input for one driver call.
The core package does not prescribe merchant, store, product, user, quantity,
or payment fields and performs no JSON round trip. The caller and driver
implementation agree on its dynamic type. Callers, drivers, and constraint
evaluators must treat the value and everything reachable from it as immutable
and must not retain it after the call.

`DriverRequest` groups `BenefitReference` and `DriverInput`; each
operation-specific request type embeds it by value.

`Inspect` returns a normalized snapshot. `BenefitInfo.DriverType` identifies the
driver; provider and kind metadata come from its `DriverDescriptor`.
`ProviderBenefitId` is optional and must not contain a bearer secret. Drivers map
raw provider states to normalized `Status` values and may retain non-standard
diagnostics in opaque `ProviderData`. When a reference does not resolve to a
benefit, `Inspect` returns `ErrBenefitNotFound` and no `BenefitInfo`. This lookup
outcome is not a benefit lifecycle status.

`Evaluate` is a non-consuming quote or preflight. It is useful for displaying a
projected outcome and for preparing a multi-benefit plan, but callers never have
to call it before `Redeem`. `Redeem` is authoritative, accepts an empty
`EvaluationToken`, and must recheck current status, provider matching, and all
constraints even when an earlier evaluation was eligible. A confirmed missing
benefit is represented by `EvaluationFailureBenefitNotFound` from `Evaluate`
and `RedeemFailureBenefitNotFound` from `Redeem`, not by a Go error.

`DriverDescriptor.Operations` declares the driver type's maximum optional
`OperationCapabilities`. Presence means supported; there is no separate
supported flag. Modes are explicit: a Reverse capability must contain `full`
and may additionally contain `partial`. A driver that declares Reverse must
implement `Reverser`.

`BenefitInfo.OperationPolicies` contains sparse, benefit-specific exceptions to
those driver capabilities. With no policies, the benefit inherits the complete
driver capability set. A policy with `Disabled` permanently disables its matched
operation or modes. A conditional policy instead supplies positive availability
constraints: all constraints from all matching policies must be satisfied.
`MatchModes` is a selector; an empty list matches the whole operation, while a
non-empty list matches only the listed modes. Policies cannot introduce an
operation or mode outside the driver's capabilities.

`EvaluateOperation` combines capabilities and policies for one operation and
optional mode. A missing capability or matching permanent disablement produces
`unsupported`; an unsatisfied conditional policy produces `ineligible`; and
satisfied constraints produce `eligible`. Raw policy constraints and operator
remarks are not part of the returned `OperationDecision`.

## Constraints

Constraints and operation capabilities are ordered lists. A concurrency-safe
`ConstraintRegistry` maps each namespaced `ConstraintType` to its evaluator.
Registration validates the type name; evaluation does not reject malformed
input names separately because any unregistered type is already
`unrecognized`.

All constraints are evaluated so callers receive every violation. An unknown,
invalid, errored, or normally unsatisfied constraint makes the aggregate report
unsatisfied. `ConstraintReport.Status` distinguishes `unevaluated`, `satisfied`,
and `unsatisfied`; an evaluated empty list is satisfied. A satisfied report has
no decisions, while an unsatisfied report contains only its `Violations`.

Constraint definitions contain evaluator parameters and an optional operator
remark. They are available in-process and to explicit management mappings, but
are excluded from `BenefitInfo` JSON. A `ConstraintDecision` contains only the
constraint type, its decision code, and optional safe diagnostic information;
it never embeds the original definition. Human-facing constraint information
belongs in `Notice` instead.

Drivers should model fixed filters compiled from `DriverConfig` and per-call
facts extracted from `DriverInput` with the same constraint mechanism. A
driver should return a merchant, store, or product mismatch as a
`constraint.unsatisfied` business result, not as an unsupported operation.

The default registry includes context-independent time-range, weekday, and
redemption-limit evaluators. Amount and scope evaluators require
application-defined extractors so the package does not need a universal
transaction structure. A currency-wildcard amount constraint applies the same
integer threshold in the actual currency's minor-unit convention; it does not
perform precision or exchange-rate conversion.

## Outcomes and mutation results

`BenefitOutcome` describes the value projected by Evaluate or confirmed by
Redeem. Its current normalized component is an optional monetary discount; the
container can gain grant or fulfillment components without changing driver
method signatures.

`ReversalOutcome` separately describes what a confirmed Reverse undid and what
benefit resources it restored. `Reversed` contains the part of the delivered
`BenefitOutcome` that was undone, while `Restored` contains a
`BenefitRestoration`. Its current normalized component is an optional monetary
balance or allowance. These values are intentionally separate: undoing a
transaction effect does not necessarily make the consumed benefit resource
available again, and restoring a benefit does not imply a monetary refund.
The restored amount is not required to equal the reversed discount amount.

All monetary values use integer minor units. Major/minor-unit conversion,
currency precision, exchange rates, and validation against the assigned ISO
4217 code list are application responsibilities; the core package carries no
currency registry. A non-empty `Currency` must have the three-ASCII-letter ISO
4217 form. Input and comparison are case-insensitive, while applications and
drivers should emit the canonical upper-case form.

The meaning of an empty currency is field-specific and documented on its
containing type. Drivers should resolve and emit currencies on durable outcomes
when possible so the records remain self-contained. A `free` discount means the
payable amount is zero and the discount equals the original amount.

State-changing results distinguish confirmed success, confirmed failure,
accepted but pending work, and an unknown outcome. Provider timeouts must not be
reported as confirmed failures when the remote operation may have completed.
Go errors are reserved for local invocation or integration failures that do not
express a confirmed provider business result.

Failure `Code` values are stable machine identifiers for program logic and
client localization. `Diagnostic.Reason` and `Diagnostic.Details` contain
optional occurrence-specific troubleshooting information; they are not stable
or localized, must not drive logic, and must not be shown directly to end
users. Diagnostic details must contain only non-sensitive data. Notice codes
follow the same stable-localization principle, while notice text is user- or
operator-facing information and carries its BCP 47 language tag in `Lang`.

`RedemptionId` and `ReversalId` are optional caller-assigned correlation and
idempotency IDs. When supplied, callers reuse the same ID when retrying an
uncertain operation. Drivers pass it to providers as an idempotency key when
possible, while hosts may persist or deduplicate operations when a provider has
no idempotency facility. Without an operation ID, the caller and driver can
only rely on provider-side idempotency. In an idempotent implementation, a
same-ID retry returns the original result rather than `benefit.redeemed`; the
core package does not store operation results.

Provider-assigned IDs are separate from caller-assigned operation IDs. Redeem
returns a `ProviderRedemptionId` when the provider supplies one. Reverse accepts
that ID together with the required `BenefitReference`, allowing a driver to
address one provider redemption. Drivers that reverse the benefit as a whole
may ignore all redemption IDs and use only `BenefitReference`.

`benefit.redeemed` means a new operation targeted an already consumed benefit;
`benefit.exhausted` means no usage, amount, or quantity remains.
`reversal.unsupported` means reversal capability is unavailable,
`redemption.irreversible` applies to one specific redemption, and
`reversal.window_expired` identifies an elapsed reversal window.

## Composition and security

The package processes exactly one benefit per driver call. A future
orchestration layer can select automatic and explicit benefits, plan their
order, optionally assign one operation ID per atomic redemption, and compensate
completed steps with Reverse. A fixed exchange that one provider performs
atomically may be modeled as Redeem; an exchange spanning independent drivers
belongs in the orchestration layer.

Applications must encrypt stored driver configurations, mask secrets in
management APIs, and protect bearer references with an appropriate lookup and
storage strategy. `ProviderData` is opaque and may contain provider-specific
diagnostics, so it must be reviewed before being exposed outside the trusted
application boundary.

## Naming and JSON

Package-defined identifiers use camel case and treat initialisms like ordinary
words: `Id`, `Url`, `Http`, `Json`, and `Api`, rather than `ID`, `URL`, `HTTP`,
`JSON`, and `API`. This convention applies consistently to types, functions,
methods, fields, constants, variables, and parameters defined by the package.
Names from imported packages retain their original spelling.

Public structs use their exported Go field names as the default JSON field
names. JSON tags control only omission or exclusion, such as `json:",omitempty"`,
`json:",omitzero"`, and `json:"-"`; they do not define alternative field names.
JSON objects supplied by applications or providers, including driver configs,
constraint params, diagnostic detail keys, and opaque provider data, keep their
own protocol-defined naming.

## Development

```sh
go test -race ./...
go vet ./...
```
