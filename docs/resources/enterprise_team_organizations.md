---
page_title: "github_enterprise_team_organizations (Resource)"
description: |-
  Authoritatively manages all organization assignments for a team in selected mode. Omitted organizations are removed. Use only one instance per team; import existing assignments first. Creation and updates require GitHub to return organization_selection_type so selected mode can be verified.
---

# github_enterprise_team_organizations (Resource)

Authoritatively manages all organization assignments for a team in selected mode. Omitted organizations are removed. Use only one instance per team; import existing assignments first. Creation and updates require GitHub to return organization_selection_type so selected mode can be verified.

## Example

```hcl
resource "github_enterprise_team_organizations" "example" {
  enterprise_slug = "my-enterprise"

  team_id            = github_enterprise_team.example.team_id
  organization_slugs = ["my-organization"]
}
```

## Schema

| Attribute | Type | Requirement | Description |
| --- | --- | --- | --- |
| `enterprise_slug` | `String` | Required | Enterprise account slug. |
| `id` | `String` | Read-only | enterprise_slug/team_id. |
| `organization_slugs` | `Set of String` | Required | Complete set of lowercase organization slugs. An empty set removes all assignments. |
| `team_id` | `Number` | Required | Numeric team ID. |

## Import

```sh
terraform import github_enterprise_team_organizations.example my-enterprise/123456
```

Uses the GitHub provider authentication. Team resources require enterprise access. Existing imports use the same numeric identifiers as `vmvarela/github-enterprise`; changing provider configuration and supplying `enterprise_slug` does not require recreating teams.
