# Quality Lab Engineering Rules

## Contract Ownership

- Public interface identifiers in `desc/api/commerce.api` are represented as strings; this keeps the external contract independent from internal numeric storage.
- Interface changes start from `desc/api/commerce.api`. Generated files such as `internal/handler/commerce/catalog.go` and `internal/types/commerce/types.go` must be regenerated and must not be edited directly.

## Tenant Boundary

- The catalog and order capabilities in `internal/catalog/service.go` and `internal/order/service.go` require a tenant context validated by `internal/tenant/context.go`. A caller must not accept a tenant identifier from payload data after the request context has been established.
- Repository operations in `internal/catalog/service.go` and `internal/order/service.go` must receive the validated tenant identifier explicitly; an empty tenant identifier is invalid.

## State Changes

- Order status changes must go through `order.Transition` in `internal/order/model.go`; direct assignment outside the order package is forbidden.
- A failed inventory compensation in `internal/order/service.go` is an observable inconsistency, not a successful rollback. Callers must preserve and surface that error.

## Command Safety

- Deployment and environment mutation commands may be described, but their command policy is `requires_authorization`: they must not be executed without explicit authorization in the current request.

## Evidence Format

- Rule evidence must name concrete existing files such as `desc/api/commerce.api` or `internal/order/service.go`. Directory globs are not valid evidence paths.
