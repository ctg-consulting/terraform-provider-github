---
page_title: "GitHub: github_enterprise_ruleset"
description: "Manages an enterprise repository branch ruleset."
---

# github_enterprise_ruleset

Manages a branch ruleset for named repositories in selected GitHub Enterprise organizations. The provider identity needs permission to manage enterprise rulesets.

```terraform
resource "github_enterprise_ruleset" "infrastructure" {
  enterprise_slug  = "example-enterprise"
  name             = "Infrastructure approvals"
  enforcement      = "active"
  organization_ids = [123456]

  conditions {
    repository_name {
      include = ["infrastructure"]
      exclude = []
    }
    ref_name {
      include = ["~DEFAULT_BRANCH"]
      exclude = []
    }
  }

  rules {
    pull_request {
      required_approving_review_count = 1
      dismiss_stale_reviews_on_push   = true
    }
  }
}
```

- `enterprise_slug`, `name`, `enforcement`, and `organization_ids` are required.
- `target` defaults to `branch`, the supported target.
- `conditions` requires `repository_name` and `ref_name`, each with `include` and `exclude` patterns. `repository_name` optionally supports `protected`.
- `rules` and optional `bypass_actors` use the same blocks as [organization rulesets](organization_ruleset.md), subject to GitHub's enterprise branch rules.
- `ruleset_id` and `node_id` are computed.

Import an enterprise branch ruleset that uses organization IDs and repository names with `enterprise_slug/ruleset_id`.

```shell
terraform import github_enterprise_ruleset.infrastructure example-enterprise/42
```
