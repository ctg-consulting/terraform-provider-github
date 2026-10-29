---
page_title: "github_enterprise_team_membership (Resource)"
description: |-
  Manages one direct team membership. Other members are unaffected. Do not use with IdP-managed team memberships.
---

# github_enterprise_team_membership (Resource)

Manages one direct team membership. Other members are unaffected. Do not use with IdP-managed team memberships.

## Example

```hcl
resource "github_enterprise_team_membership" "example" {
  enterprise_slug = "my-enterprise"

  team_id  = github_enterprise_team.example.team_id
  username = "octocat"
}
```

## Schema

| Attribute | Type | Requirement | Description |
| --- | --- | --- | --- |
| `enterprise_slug` | `String` | Required | Enterprise account slug. |
| `id` | `String` | Read-only | enterprise_slug/team_id/username. |
| `team_id` | `Number` | Required | Numeric team ID. Remains stable when a team is renamed. |
| `user_id` | `Number` | Read-only | Numeric GitHub user ID. |
| `username` | `String` | Required | GitHub username. |

## Import

```sh
terraform import github_enterprise_team_membership.example my-enterprise/123456/octocat
```

Uses the GitHub provider authentication. Team resources require enterprise access. Existing imports use the same numeric identifiers as `vmvarela/github-enterprise`; changing provider configuration and supplying `enterprise_slug` does not require recreating teams.
