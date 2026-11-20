package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v92/github"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceGithubEnterpriseRuleset() *schema.Resource {
	org := resourceGithubOrganizationRuleset().Schema
	orgConditions := org["conditions"].Elem.(*schema.Resource).Schema
	repoName := orgConditions["repository_name"]
	repoName.ConflictsWith = nil
	repoName.AtLeastOneOf = nil
	repoName.Optional = false
	repoName.Required = true
	repoName.MinItems = 1
	refName := orgConditions["ref_name"]
	refName.Optional = false
	refName.Required = true
	refName.MinItems = 1

	return &schema.Resource{
		Description:   "Manages a branch ruleset scoped to named repositories in selected GitHub Enterprise organizations.",
		CreateContext: resourceGithubEnterpriseRulesetCreate,
		ReadContext:   resourceGithubEnterpriseRulesetRead,
		UpdateContext: resourceGithubEnterpriseRulesetUpdate,
		DeleteContext: resourceGithubEnterpriseRulesetDelete,
		Importer:      &schema.ResourceImporter{StateContext: resourceGithubEnterpriseRulesetImport},
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, _ any) error {
			return validateRulesetRules(ctx, d)
		},
		Schema: map[string]*schema.Schema{
			"enterprise_slug": enterpriseSlugSchema(),
			"name":            org["name"],
			"target": {
				Type: schema.TypeString, Optional: true, Default: "branch",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"branch"}, false)),
				Description:      "The ruleset target. Currently supports branch rulesets.",
			},
			"enforcement":   org["enforcement"],
			"bypass_actors": org["bypass_actors"],
			"ruleset_id":    org["ruleset_id"],
			"node_id":       org["node_id"],
			"rules":         org["rules"],
			"organization_ids": {
				Type: schema.TypeSet, Required: true, MinItems: 1,
				Elem:        &schema.Schema{Type: schema.TypeInt, ValidateDiagFunc: validation.ToDiagFunc(validation.IntAtLeast(1))},
				Description: "Numeric organization IDs targeted by the ruleset.",
			},
			"conditions": {
				Type: schema.TypeList, Required: true, MinItems: 1, MaxItems: 1,
				Description: "Repository and branch names targeted by the ruleset.",
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"repository_name": repoName,
					"ref_name":        refName,
				}},
			},
		},
	}
}

func expandEnterpriseRuleset(d *schema.ResourceData) github.RepositoryRuleset {
	ruleset := resourceGithubRulesetObject(d, d.Get("enterprise_slug").(string))
	ruleset.Source = ""
	ruleset.SourceType = nil
	ids := d.Get("organization_ids").(*schema.Set).List()
	organizationIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		organizationIDs = append(organizationIDs, int64(id.(int)))
	}
	ruleset.Conditions.OrganizationID = &github.RepositoryRulesetOrganizationIDsConditionParameters{OrganizationIDs: organizationIDs}
	return ruleset
}

func enterpriseRulesetIdentity(id string) (string, int64, error) {
	slug, number, found := strings.Cut(id, "/")
	if !found {
		return "", 0, fmt.Errorf("expected enterprise_slug/ruleset_id")
	}
	value, err := strconv.ParseInt(number, 10, 64)
	if err != nil || value <= 0 || enterpriseSlugSchema().ValidateDiagFunc(slug, nil).HasError() {
		return "", 0, fmt.Errorf("expected a valid enterprise slug and positive ruleset ID")
	}
	return slug, value, nil
}

func resourceGithubEnterpriseRulesetImport(_ context.Context, d *schema.ResourceData, _ any) ([]*schema.ResourceData, error) {
	slug, _, err := enterpriseRulesetIdentity(d.Id())
	if err != nil {
		return nil, err
	}
	if err := d.Set("enterprise_slug", slug); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}

func resourceGithubEnterpriseRulesetCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	slug := d.Get("enterprise_slug").(string)
	ruleset, _, err := meta.(*Owner).v3client.Enterprise.CreateRepositoryRuleset(ctx, slug, expandEnterpriseRuleset(d))
	if err != nil {
		return diag.FromErr(err)
	}
	if ruleset == nil || ruleset.GetID() <= 0 {
		return diag.Errorf("GitHub returned no ID for the created enterprise ruleset")
	}
	d.SetId(fmt.Sprintf("%s/%d", slug, ruleset.GetID()))
	return resourceGithubEnterpriseRulesetRead(ctx, d, meta)
}

func resourceGithubEnterpriseRulesetRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	slug, id, err := enterpriseRulesetIdentity(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	ruleset, resp, err := meta.(*Owner).v3client.Enterprise.GetRepositoryRuleset(ctx, slug, id)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound && !d.IsNewResource() {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	if ruleset == nil || ruleset.GetID() != id || ruleset.Target == nil || *ruleset.Target != github.RulesetTargetBranch ||
		ruleset.Name == "" || ruleset.Enforcement == "" || ruleset.Rules == nil {
		return diag.Errorf("GitHub returned an incomplete or unsupported enterprise branch ruleset")
	}
	conditions := ruleset.Conditions
	if conditions == nil || conditions.OrganizationID == nil || len(conditions.OrganizationID.OrganizationIDs) == 0 ||
		conditions.RepositoryName == nil || conditions.RefName == nil ||
		conditions.OrganizationName != nil || conditions.OrganizationProperty != nil ||
		conditions.RepositoryID != nil || conditions.RepositoryProperty != nil {
		return diag.Errorf("enterprise ruleset must target organization IDs, repository names and branch names")
	}
	for key, value := range map[string]any{
		"enterprise_slug":  slug,
		"ruleset_id":       id,
		"node_id":          ruleset.GetNodeID(),
		"name":             ruleset.Name,
		"target":           string(*ruleset.Target),
		"enforcement":      ruleset.Enforcement,
		"bypass_actors":    flattenBypassActors(ctx, ruleset.BypassActors),
		"organization_ids": conditions.OrganizationID.OrganizationIDs,
		"conditions":       flattenConditions(ctx, conditions, true),
		"rules":            flattenRules(ctx, ruleset.Rules, true),
	} {
		if err := d.Set(key, value); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func resourceGithubEnterpriseRulesetUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	slug, id, err := enterpriseRulesetIdentity(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	if _, _, err := meta.(*Owner).v3client.Enterprise.UpdateRepositoryRuleset(ctx, slug, id, expandEnterpriseRuleset(d)); err != nil {
		return diag.FromErr(err)
	}
	return resourceGithubEnterpriseRulesetRead(ctx, d, meta)
}

func resourceGithubEnterpriseRulesetDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	slug, id, err := enterpriseRulesetIdentity(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = meta.(*Owner).v3client.Enterprise.DeleteRepositoryRuleset(ctx, slug, id)
	var response *github.ErrorResponse
	if errors.As(err, &response) && response.Response.StatusCode == http.StatusNotFound {
		return nil
	}
	return diag.FromErr(err)
}
