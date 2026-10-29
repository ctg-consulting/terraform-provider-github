package github

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	client "github.com/integrations/terraform-provider-github/v6/internal/enterpriseteams"
)

func resourceGithubEnterpriseTeamMembership() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages one direct enterprise-team member; other members are untouched. Do not use for IdP-managed membership.",
		CreateContext: resourceGithubEnterpriseTeamMembershipCreate,
		ReadContext:   resourceGithubEnterpriseTeamMembershipRead,
		DeleteContext: resourceGithubEnterpriseTeamMembershipDelete,
		Importer:      &schema.ResourceImporter{StateContext: enterpriseTeamImport(3)},
		Schema: map[string]*schema.Schema{
			"enterprise_slug": enterpriseSlugSchema(),
			"team_id":         enterpriseTeamIDSchema(),
			"username": {Type: schema.TypeString, Required: true, ForceNew: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringMatch(regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`), "must be a GitHub username"))},
			"user_id": {Type: schema.TypeInt, Computed: true},
		},
	}
}

func resourceGithubEnterpriseTeamMembershipCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	enterprise, id, username := d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int)), d.Get("username").(string)
	team, err := c.FindTeam(ctx, enterprise, id, "")
	if err != nil {
		return diag.FromErr(err)
	}
	if team.GroupID != "" {
		return diag.Errorf("direct membership cannot be managed for an IdP-linked enterprise team")
	}
	u, err := c.AddMember(ctx, enterprise, id, username)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("%s/%d/%s", enterprise, id, username))
	return diag.FromErr(d.Set("user_id", u.ID))
}

func resourceGithubEnterpriseTeamMembershipRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	u, err := c.Membership(ctx, d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int)), d.Get("username").(string))
	if client.IsNotFound(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	return diag.FromErr(d.Set("user_id", u.ID))
}

func resourceGithubEnterpriseTeamMembershipDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	err = c.RemoveMember(ctx, d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int)), d.Get("username").(string))
	if err != nil && !client.IsNotFound(err) {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
