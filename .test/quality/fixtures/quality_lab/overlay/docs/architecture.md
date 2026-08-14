# Quality Lab Architecture

The HTTP layer is generated from `desc/api`. Handwritten application capabilities live under the focused packages in `internal/`; generated transport code is not a business capability owner.

Catalog reads require tenant validation. Order placement validates the tenant, reserves inventory, records the order, and publishes an event. Inventory and order repositories are independent boundaries. A failed order write triggers compensation, but failed compensation leaves an inconsistency that the caller must handle.

The dispatcher owns its queue lifecycle. Producers receive an explicit full or closed error instead of assuming delivery.
