# Quality Lab Fixture

`quality_lab` is generated fresh for every quality run.

- `build.sh` invokes `jzero new`, copies fixture inputs and handwritten code, then invokes `jzero gen`.
- `inputs/` contains only generator inputs such as `.api` descriptions.
- `overlay/` contains authoritative project rules, architecture context, and handwritten source scenarios.
- Generated handlers, types, routes, Swagger files, and framework scaffolding exist only under `.test/quality/runtime/`.

The scenarios cover both desired knowledge and deliberate noise:

- authoritative interface ownership and identifier contracts;
- tenant ownership as a capability prerequisite;
- explicit order state transitions;
- independent inventory and order writes with a compensation failure window;
- bounded queue overload and lifecycle behavior;
- reusable module-level capability entries;
- generated transport shells and ordinary helpers that should not become primary capability entries;
- broad audit vocabulary that must not create a synthetic business domain.
- payment idempotency and the uncertain boundary between provider success and local persistence;
- role-owned approval stages with terminal states and a finance threshold;
- exclusive best-benefit pricing with a final discount cap;
- fulfillment that returns partial allocations without pretending they were rolled back;
- cumulative refund limits, replay reuse, and uncertain provider outcomes;
- subscription renewal dates, grace-period service, and post-grace suspension;
- warehouse shipment grouping with partial and rejected line quantities preserved.

The fixture is not a golden Skill. The generated wording may vary as long as the resulting Skill preserves the configured decisions, evidence, and boundaries.
