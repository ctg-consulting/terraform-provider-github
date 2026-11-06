package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	client "github.com/integrations/terraform-provider-github/v6/internal/enterpriseteams"
)

// This endpoint requires a GitHub App installed on the enterprise with the
// enterprise_organization_installations write permission. An organization
// installation token or personal access token cannot manage it.
func resourceGithubEnterpriseOrganizationAppInstallation() *schema.Resource {
	return &schema.Resource{
		Description:   "Installs a GitHub App on an enterprise-owned organization with access to all repositories.",
		CreateContext: enterpriseOrganizationAppInstallationCreate,
		ReadContext:   enterpriseOrganizationAppInstallationRead,
		UpdateContext: enterpriseOrganizationAppInstallationUpdate,
		DeleteContext: enterpriseOrganizationAppInstallationDelete,
		Importer:      &schema.ResourceImporter{StateContext: enterpriseOrganizationAppInstallationImport},
		Schema: map[string]*schema.Schema{
			"enterprise_slug": enterpriseSlugSchema(),
			"organization": {Type: schema.TypeString, Required: true, ForceNew: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace)},
			"client_id": {Type: schema.TypeString, Required: true, ForceNew: true,
				Description: "Client ID of the App to install (not the managing App)."},
			"repository_selection": {Type: schema.TypeString, Required: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"all"}, false))},
			"app_slug":        {Type: schema.TypeString, Computed: true},
			"installation_id": {Type: schema.TypeInt, Computed: true},
		},
	}
}

func enterpriseOrganizationAppInstallationImport(_ context.Context, d *schema.ResourceData, _ any) ([]*schema.ResourceData, error) {
	parts := strings.Split(d.Id(), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("expected import ID enterprise/organization/installation_id")
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("expected positive installation ID in enterprise/organization/installation_id")
	}
	for key, value := range map[string]any{"enterprise_slug": parts[0], "organization": parts[1], "installation_id": id} {
		if err := d.Set(key, value); err != nil {
			return nil, err
		}
	}
	return []*schema.ResourceData{d}, nil
}

func enterpriseOrganizationAppInstallationCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	installation, err := c.InstallApp(ctx, d.Get("enterprise_slug").(string), d.Get("organization").(string), d.Get("client_id").(string))
	if err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("installation_id", installation.ID); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("%s/%s/%d", d.Get("enterprise_slug"), d.Get("organization"), installation.ID))
	return enterpriseOrganizationAppInstallationRead(ctx, d, meta)
}

func enterpriseOrganizationAppInstallationRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	installations, err := c.ListAppInstallations(ctx, d.Get("enterprise_slug").(string), d.Get("organization").(string))
	if err != nil {
		return diag.FromErr(err)
	}
	id := int64(d.Get("installation_id").(int))
	for _, installation := range installations {
		if installation.ID != id {
			continue
		}
		if err := d.Set("app_slug", installation.AppSlug); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("repository_selection", installation.RepositorySelection); err != nil {
			return diag.FromErr(err)
		}
		return nil
	}
	d.SetId("")
	return nil
}

func enterpriseOrganizationAppInstallationUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	installation, err := c.InstallApp(ctx, d.Get("enterprise_slug").(string), d.Get("organization").(string), d.Get("client_id").(string))
	if err != nil {
		return diag.FromErr(err)
	}
	if installation.ID != int64(d.Get("installation_id").(int)) {
		return diag.Errorf("GitHub returned a different installation ID; refresh before retrying")
	}
	return enterpriseOrganizationAppInstallationRead(ctx, d, meta)
}

func enterpriseOrganizationAppInstallationDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	err = c.UninstallApp(ctx, d.Get("enterprise_slug").(string), d.Get("organization").(string), int64(d.Get("installation_id").(int)))
	if err != nil && !client.IsNotFound(err) {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
