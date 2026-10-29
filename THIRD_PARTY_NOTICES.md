# Third-party notices

`internal/enterpriseteams/client.go` and `client_test.go` are adapted from
[vmvarela/terraform-provider-github-enterprise](https://github.com/vmvarela/terraform-provider-github-enterprise)
at commit `5fd48733c50cb89e3fd64db3e566780ddea0d7d0`, under the MIT license
included in that directory. The client now reuses this provider's authenticated
HTTP transport. Enterprise-team resource behavior follows that implementation,
ported to the Terraform Plugin SDK used by this provider.
