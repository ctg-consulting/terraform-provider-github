---
page_title: "GitHub: github_branch_protection_review_requirements"
description: "Manages approval requirements on an existing classic branch protection rule."
---

# github_branch_protection_review_requirements

Manages approval requirements on a classic branch protection rule that already
requires pull requests. The pull request requirement, status checks, and other
branch protections remain in effect. Removing this resource from Terraform leaves
the rule and its last applied settings in GitHub. Use one Terraform resource to
manage the approval requirements on any given rule.

```terraform
resource "github_branch_protection_review_requirements" "default" {
  repository                      = "example"
  pattern                         = "main"
  required_approving_review_count = 0
  require_code_owner_reviews      = false
  require_last_push_approval      = false
}
```

All arguments are required. Specify the exact configured pattern, including any
wildcards. The approval count accepts values from 0 to 6. Set the count to 0
and both approval flags to false to keep the pull request requirement while
making reviews optional.

Import existing approval settings using the repository name and rule pattern.

```terraform
import {
  to = github_branch_protection_review_requirements.default
  id = "example:main"
}
```
