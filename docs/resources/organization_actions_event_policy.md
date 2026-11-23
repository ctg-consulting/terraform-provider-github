---
page_title: "GitHub: github_organization_actions_event_policy"
description: "Restricts workflow events for repositories selected by visibility."
---

# github_organization_actions_event_policy

Manages an organization Actions execution policy with one event allowlist and a
repository visibility condition. The provider owner selects the organization.
The credential needs organization Administration write permissions.

```terraform
resource "github_organization_actions_event_policy" "public" {
  name                  = "Public workflow events"
  enforcement           = "active"
  repository_visibility = "public"
  allowed_events        = ["pull_request", "push"]
}
```

All arguments are required. Enforcement accepts `active`, `disabled`, or
`evaluate`. Repository visibility accepts `public`, `private`, or `internal`.
`allowed_events` is a nonempty set of supported Actions event names. Events
outside the set cannot trigger workflows in the targeted repositories when the
policy is active.

Import a standalone event allowlist with the supported visibility condition
using its numeric policy ID.

```shell
terraform import github_organization_actions_event_policy.public 42
```
