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

Management applications use `ConfigSchema` to render configuration forms and
call `ValidateConfig` before encrypting and persisting operator input. A
`DriverConfig` is either empty, when the schema permits it, or a UTF-8 JSON
object. Schemas use JSON Schema Draft 2020-12 and may include UI extensions such
as `x-secret`. Runtime binding does not call management validation again, so
`CompileConfig` must independently validate and parse persisted configuration.

At runtime, `DriverRegistry.Bind` calls deterministic, local `CompileConfig`
and creates a new lightweight driver. The resulting driver has provider
credentials and fixed matching rules already bound, so operation calls do not
repeatedly receive sensitive configuration. Implementations must make compiled
factories and bound drivers safe for concurrent use.

## Operation inputs and behavior

`BenefitReference` is an opaque, optional reference such as a bearer coupon code
or provider record ID. It is not a provider API credential and is excluded from
default JSON serialization. Applications should authenticate requests and
resolve device, merchant, or user facts before constructing driver requests.

`OperationContext` carries application-defined, in-process facts. The core
package does not prescribe merchant, store, product, user, quantity, or payment
fields and performs no JSON round trip. Callers, drivers, and constraint
evaluators must treat the value and everything reachable from it as immutable
and must not retain it after the call.

`Inspect` returns a normalized snapshot. `BenefitInfo.DriverType` identifies the
driver; provider and kind metadata come from its `DriverDescriptor`.
`ProviderBenefitID` is optional and must not contain a bearer secret. Drivers map
raw provider states to normalized `Status` values and may retain non-standard
diagnostics in opaque `ProviderData`.

`Evaluate` is a non-consuming quote or preflight. It is useful for displaying a
projected outcome and for preparing a multi-benefit plan, but callers never have
to call it before `Redeem`. `Redeem` is authoritative, accepts an empty
`EvaluationToken`, and must recheck current status, provider matching, and all
constraints even when an earlier evaluation was eligible.

A supported `Reverse` operation always supports full reversal. Partial reversal
is supported only when its declared modes contain `partial`; an empty mode list
therefore means full-only. A driver that declares Reverse must implement
`Reverser`.

`OperationSupport` is an internal capability definition. `EvaluateOperation`
returns an `OperationDecision` whose mutually exclusive status is `unsupported`,
`ineligible`, or `eligible`; raw operation constraints and operator remarks are
not part of that decision.

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
facts extracted from `OperationContext` with the same constraint mechanism. A
driver should return a merchant, store, or product mismatch as a
`constraint.unsatisfied` business result, not as an unsupported operation.

The default registry includes context-independent time-range, weekday, and
redemption-limit evaluators. Amount and scope evaluators require
application-defined extractors so the package does not need a universal
transaction structure.

## Outcomes and mutation results

`BenefitOutcome` describes the value projected by Evaluate or confirmed by
Redeem. Its current normalized component is an optional monetary discount; the
container can gain grant or fulfillment components without changing driver
method signatures.

All monetary values use integer minor units. Currency lookup and major/minor
conversion are provided by
[`go-currency`](https://github.com/xgfone/go-currency). A `free` discount means
the payable amount is zero and the discount equals the original amount.

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
operator-facing information and carries its BCP 47 language tag in `lang`.

Callers assign a unique `RedemptionID` or `ReversalID` before each mutation and
reuse the same ID when retrying an uncertain operation. Drivers and hosts are
responsible for implementing idempotency: drivers pass the ID to providers as
an idempotency key when possible, while hosts must persist or deduplicate
operations when a provider has no idempotency facility. In an idempotent
implementation, a same-ID retry returns the original result rather than
`benefit.redeemed`; the core package does not store operation results.

`benefit.redeemed` means a new operation targeted an already consumed benefit;
`benefit.exhausted` means no usage, amount, or quantity remains.
`reversal.unsupported` means reversal capability is unavailable,
`redemption.irreversible` applies to one specific redemption, and
`reversal.window_expired` identifies an elapsed reversal window.

## Composition and security

The package processes exactly one benefit per driver call. A future
orchestration layer can select automatic and explicit benefits, plan their
order, assign one operation ID per atomic redemption, and compensate completed
steps with Reverse. A fixed exchange that one provider performs atomically may
be modeled as Redeem; an exchange spanning independent drivers belongs in the
orchestration layer.

Applications must encrypt stored driver configurations, mask secrets in
management APIs, and protect bearer references with an appropriate lookup and
storage strategy. `ProviderData` is opaque and may contain provider-specific
diagnostics, so it must be reviewed before being exposed outside the trusted
application boundary.

## Development

```sh
go test -race ./...
go vet ./...
```
