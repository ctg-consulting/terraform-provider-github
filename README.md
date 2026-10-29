# CTG GitHub Terraform Provider

CTG Consulting's fork of [integrations/terraform-provider-github](https://github.com/integrations/terraform-provider-github).
Based on upstream `main` at `1b9c5c13eb840a72c99d088f3bf3b5e00b950c04`, including unreleased fixes.

This fork additionally supports enterprise teams (`BusinessTeam`) as bypass actors
in both organization and repository rulesets. Organization teams use `Team`.

```hcl
terraform {
  required_providers {
    github = {
      source  = "ctg-consulting/github"
      version = "6.14.0-ctg.1"
    }
  }
}
```

Use this source in every module that requires the GitHub provider. Authentication
and resource names are unchanged from upstream. Releases are signed using the public
key in `signing-key.asc` and published through the Terraform Registry.

Run `go test ./... -skip '^TestAcc'` for unit tests. The focused regression test is
`go test ./github -run TestBusinessTeamRulesetBypass`.

The `ctg` branch carries CTG changes; `main` tracks the upstream baseline.
Upstream documentation is retained in `docs/`.
