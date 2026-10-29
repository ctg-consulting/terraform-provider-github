package github

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	client "github.com/integrations/terraform-provider-github/v6/internal/enterpriseteams"
)

func resourceGithubEnterpriseTeamOrganizations() *schema.Resource {
	return &schema.Resource{
		Description:   "Authoritatively manages selected organization assignments. Import existing assignments before managing them. An empty set removes all assignments.",
		CreateContext: resourceGithubEnterpriseTeamOrganizationsCreate,
		ReadContext:   resourceGithubEnterpriseTeamOrganizationsRead,
		UpdateContext: resourceGithubEnterpriseTeamOrganizationsUpdate,
		DeleteContext: resourceGithubEnterpriseTeamOrganizationsDelete,
		Importer:      &schema.ResourceImporter{StateContext: enterpriseTeamImport(2)},
		Schema: map[string]*schema.Schema{
			"enterprise_slug": enterpriseSlugSchema(),
			"team_id":         enterpriseTeamIDSchema(),
			"organization_slugs": {Type: schema.TypeSet, Required: true, Elem: &schema.Schema{
				Type: schema.TypeString, ValidateDiagFunc: validation.ToDiagFunc(validation.StringMatch(regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`), "must be a lowercase organization slug"))}},
		},
	}
}

func enterpriseSelectedTeam(ctx context.Context, c *client.Client, enterprise string, id int64) error {
	team, err := c.FindTeam(ctx, enterprise, id, "")
	if err != nil {
		return err
	}
	if team.OrganizationSelectionType == nil || *team.OrganizationSelectionType != "selected" {
		return fmt.Errorf("team %d must have a verified organization_selection_type of selected", id)
	}
	return nil
}

func enterpriseOrganizationNames(orgs []client.Organization) []string {
	names := make([]string, 0, len(orgs))
	for _, o := range orgs {
		names = append(names, strings.ToLower(o.Login))
	}
	sort.Strings(names)
	return names
}

func reconcileEnterpriseOrganizations(ctx context.Context, c *client.Client, d *schema.ResourceData, current []string) error {
	existing := map[string]bool{}
	for _, name := range current {
		existing[name] = true
	}
	desired := map[string]bool{}
	for _, name := range d.Get("organization_slugs").(*schema.Set).List() {
		desired[name.(string)] = true
	}
	var add, remove []string
	for name := range desired {
		if !existing[name] {
			add = append(add, name)
		}
	}
	for name := range existing {
		if !desired[name] {
			remove = append(remove, name)
		}
	}
	sort.Strings(add)
	sort.Strings(remove)
	enterprise, id := d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int))
	if err := c.ChangeOrganizations(ctx, enterprise, id, "add", add); err != nil {
		return err
	}
	return c.ChangeOrganizations(ctx, enterprise, id, "remove", remove)
}

func resourceGithubEnterpriseTeamOrganizationsCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	enterprise, id := d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int))
	if err := enterpriseSelectedTeam(ctx, c, enterprise, id); err != nil {
		return diag.FromErr(err)
	}
	current, err := c.Organizations(ctx, enterprise, id)
	if err != nil {
		return diag.FromErr(err)
	}
	if len(current) != 0 {
		return diag.Errorf("import existing enterprise team organization assignments before managing them")
	}
	d.SetId(fmt.Sprintf("%s/%d", enterprise, id))
	if err := reconcileEnterpriseOrganizations(ctx, c, d, nil); err != nil {
		diagnostics := diag.FromErr(err)
		return append(diagnostics, resourceGithubEnterpriseTeamOrganizationsRead(ctx, d, meta)...)
	}
	return resourceGithubEnterpriseTeamOrganizationsRead(ctx, d, meta)
}

func resourceGithubEnterpriseTeamOrganizationsRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	orgs, err := c.Organizations(ctx, d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int)))
	if client.IsNotFound(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	return diag.FromErr(d.Set("organization_slugs", enterpriseOrganizationNames(orgs)))
}

func resourceGithubEnterpriseTeamOrganizationsUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	enterprise, id := d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int))
	if err := enterpriseSelectedTeam(ctx, c, enterprise, id); err != nil {
		return diag.FromErr(err)
	}
	orgs, err := c.Organizations(ctx, enterprise, id)
	if err != nil {
		return diag.FromErr(err)
	}
	if err := reconcileEnterpriseOrganizations(ctx, c, d, enterpriseOrganizationNames(orgs)); err != nil {
		diagnostics := diag.FromErr(err)
		return append(diagnostics, resourceGithubEnterpriseTeamOrganizationsRead(ctx, d, meta)...)
	}
	return resourceGithubEnterpriseTeamOrganizationsRead(ctx, d, meta)
}

func resourceGithubEnterpriseTeamOrganizationsDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	enterprise, id := d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int))
	if err := enterpriseSelectedTeam(ctx, c, enterprise, id); err != nil {
		if client.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	orgs, err := c.Organizations(ctx, enterprise, id)
	if client.IsNotFound(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	err = c.ChangeOrganizations(ctx, enterprise, id, "remove", enterpriseOrganizationNames(orgs))
	if err != nil && !client.IsNotFound(err) {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
