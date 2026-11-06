---
subcategory: "Enterprise"
description: |-
  Installs a GitHub App on an enterprise-owned organization.
---

# github_enterprise_organization_app_installation

Installs a GitHub App on an enterprise-owned organization with access to all repositories. The provider must authenticate as a separate GitHub App installed on the enterprise with **Enterprise organization installations: write** permission. An organization installation token or personal access token cannot call this API.

The resource calls GitHub's enterprise organization installations endpoint. Removing it from Terraform configuration uninstalls the App. Use `lifecycle.prevent_destroy` if the installation must be retained.

## Example Usage

```terraform
resource "github_enterprise_organization_app_installation" "elt" {
  provider             = github.enterprise_app
  enterprise_slug      = "example-enterprise"
  organization         = "example-org"
  client_id            = "Iv23example"
  repository_selection = "all"

  lifecycle {
    prevent_destroy = true
  }
}
```

## Argument Reference

* `enterprise_slug` - (Required) Slug of the enterprise that owns the organization.
* `organization` - (Required) Organization where the App will be installed.
* `client_id` - (Required) Client ID of the App being installed, not the managing App.
* `repository_selection` - (Required) Must be `all`.

## Attribute Reference

* `app_slug` - Slug of the installed App.
* `installation_id` - Numeric ID of the organization installation.

## Import

Import with `enterprise_slug/organization/installation_id`, for example:

```sh
terraform import github_enterprise_organization_app_installation.elt example-enterprise/example-org/42
```
