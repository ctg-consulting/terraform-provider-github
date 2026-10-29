package github

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	client "github.com/integrations/terraform-provider-github/v6/internal/enterpriseteams"
)

func enterpriseTeamClient(meta any) (*client.Client, error) {
	owner := meta.(*Owner)
	return client.NewWithHTTPClient(owner.v3client.BaseURL(), owner.v3client.Client())
}

func enterpriseSlugSchema() *schema.Schema {
	return &schema.Schema{Type: schema.TypeString, Required: true, ForceNew: true,
		Description:      "Enterprise account slug.",
		ValidateDiagFunc: validation.ToDiagFunc(validation.StringMatch(regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`), "must be a lowercase enterprise slug"))}
}

func enterpriseTeamIDSchema() *schema.Schema {
	return &schema.Schema{Type: schema.TypeInt, Required: true, ForceNew: true,
		Description: "Stable numeric enterprise team ID.", ValidateDiagFunc: validation.ToDiagFunc(validation.IntAtLeast(1))}
}

func enterpriseTeamImport(partsCount int) schema.StateContextFunc {
	return func(_ context.Context, d *schema.ResourceData, _ any) ([]*schema.ResourceData, error) {
		parts := strings.Split(d.Id(), "/")
		if len(parts) != partsCount || !regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(parts[0]) {
			return nil, fmt.Errorf("expected enterprise_slug/team_id or enterprise_slug/team_id/username")
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("team_id must be a positive integer")
		}
		if partsCount == 3 && !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`).MatchString(parts[2]) {
			return nil, fmt.Errorf("invalid GitHub username")
		}
		if err := d.Set("enterprise_slug", parts[0]); err != nil {
			return nil, err
		}
		if err := d.Set("team_id", id); err != nil {
			return nil, err
		}
		if partsCount == 3 {
			if err := d.Set("username", parts[2]); err != nil {
				return nil, err
			}
		}
		return []*schema.ResourceData{d}, nil
	}
}

func resourceGithubEnterpriseTeam() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages an enterprise team by stable numeric identity. Memberships and organization assignments are separate resources.",
		CreateContext: resourceGithubEnterpriseTeamCreate,
		ReadContext:   resourceGithubEnterpriseTeamRead,
		UpdateContext: resourceGithubEnterpriseTeamUpdate,
		DeleteContext: resourceGithubEnterpriseTeamDelete,
		Importer:      &schema.ResourceImporter{StateContext: enterpriseTeamImport(2)},
		CustomizeDiff: customdiff.ComputedIf("slug", func(_ context.Context, d *schema.ResourceDiff, _ any) bool { return d.HasChange("name") }),
		Schema: map[string]*schema.Schema{
			"enterprise_slug":             enterpriseSlugSchema(),
			"name":                        {Type: schema.TypeString, Required: true, ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace)},
			"description":                 {Type: schema.TypeString, Optional: true, Default: ""},
			"group_id":                    {Type: schema.TypeString, Optional: true, Default: "", Description: "IdP group ID. Empty unlinks the group; unchanged links are not resent."},
			"organization_selection_type": {Type: schema.TypeString, Optional: true, Default: "disabled", ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"disabled", "selected", "all"}, false))},
			"notification_setting":        {Type: schema.TypeString, Optional: true, Default: "notifications_enabled", ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"notifications_enabled", "notifications_disabled"}, false))},
			"team_id":                     {Type: schema.TypeInt, Computed: true},
			"slug":                        {Type: schema.TypeString, Computed: true},
		},
	}
}

func enterpriseTeamBody(d *schema.ResourceData) map[string]any {
	return map[string]any{"name": d.Get("name"), "description": d.Get("description"),
		"organization_selection_type": d.Get("organization_selection_type"), "notification_setting": d.Get("notification_setting")}
}

func resourceGithubEnterpriseTeamCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	body := enterpriseTeamBody(d)
	if group := d.Get("group_id").(string); group != "" {
		body["group_id"] = group
	}
	t, err := c.CreateTeam(ctx, d.Get("enterprise_slug").(string), body)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("%s/%d", d.Get("enterprise_slug"), t.ID))
	if err := d.Set("team_id", t.ID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("slug", t.Slug); err != nil {
		return diag.FromErr(err)
	}
	return resourceGithubEnterpriseTeamRead(ctx, d, meta)
}

func resourceGithubEnterpriseTeamRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	t, err := c.FindTeam(ctx, d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int)), d.Get("slug").(string))
	if client.IsNotFound(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	if t.OrganizationSelectionType == nil || t.NotificationSetting == nil {
		return diag.Errorf("GitHub omitted enterprise team organization_selection_type or notification_setting; retaining state")
	}
	for key, value := range map[string]any{"team_id": t.ID, "name": t.Name, "slug": t.Slug,
		"description": t.Description, "group_id": t.GroupID, "organization_selection_type": *t.OrganizationSelectionType,
		"notification_setting": *t.NotificationSetting} {
		if err := d.Set(key, value); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func resourceGithubEnterpriseTeamUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	enterprise, id := d.Get("enterprise_slug").(string), int64(d.Get("team_id").(int))
	t, err := c.FindTeam(ctx, enterprise, id, d.Get("slug").(string))
	if err != nil {
		return diag.FromErr(err)
	}
	body := enterpriseTeamBody(d)
	if d.HasChange("group_id") {
		body["group_id"] = nil
		if group := d.Get("group_id").(string); group != "" {
			body["group_id"] = group
		}
	}
	updated, err := c.UpdateTeam(ctx, enterprise, t.Slug, body)
	if err != nil {
		return diag.FromErr(err)
	}
	if updated.ID != id || updated.Slug == "" {
		return diag.Errorf("unexpected enterprise team identity after update; refresh before retrying")
	}
	if err := d.Set("slug", updated.Slug); err != nil {
		return diag.FromErr(err)
	}
	return resourceGithubEnterpriseTeamRead(ctx, d, meta)
}

func resourceGithubEnterpriseTeamDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c, err := enterpriseTeamClient(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	enterprise := d.Get("enterprise_slug").(string)
	t, err := c.FindTeam(ctx, enterprise, int64(d.Get("team_id").(int)), d.Get("slug").(string))
	if client.IsNotFound(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	if err := c.DeleteTeam(ctx, enterprise, t.Slug); err != nil && !client.IsNotFound(err) {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
