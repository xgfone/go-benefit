# Go composite literal conventions

This module requires Go 1.27 or newer, as declared in `go.mod`.

- In new or modified composite literals for `InspectRequest`, `EvaluateRequest`,
  `RedeemRequest`, and `ReverseRequest`, initialize the promoted `Reference`
  and `Input` fields directly alongside the operation-specific fields.
- Do not write an explicit `DriverRequest: benefit.DriverRequest{...}`
  initializer or `DriverRequest: common`. For an existing common request,
  use `Reference: common.Reference` and `Input: common.Input`.
- Apply this convention to production code, examples, and tests. Go 1.27 supports
  promoted-field composite literals; do not reintroduce the nested form as a
  workaround for older Go syntax. Static analysis suggests simplifying that form.
- Preserve the embedded `DriverRequest` in the request type definitions. This
  rule changes construction syntax, not the type layout or serialization.

Preferred form:

```go
req := benefit.RedeemRequest{
    Reference: reference,
    Input:     input,

    RedemptionId: redemptionId,
}
```

## Embedded Diagnostic

- In composite literals for types embedding `Diagnostic`, initialize the
  promoted `Reason` and `Details` fields directly on the outer value. Do
  not use `Diagnostic: benefit.Diagnostic{...}` or `Diagnostic: diagnostic`.
- When reusing diagnostic data, write `Reason: diagnostic.Reason` and
  `Details: diagnostic.Details`. Apply this convention to production code,
  examples, and tests under the same Go 1.27+ rule as `DriverRequest`.
- Preserve the embedded `Diagnostic` in type definitions and its standalone
  construction. Only embedded-field initializers need promotion.

Preferred form:

```go
failure := benefit.RedeemFailure{
    Code: benefit.RedeemFailureProviderRejected,

    Reason:  "provider rejected the request",
    Details: map[string]any{"provider_code": 42},
}
```
