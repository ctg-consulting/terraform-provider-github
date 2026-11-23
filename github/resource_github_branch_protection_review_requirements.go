package github

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/shurcooL/githubv4"
)

func resourceGithubBranchProtectionReviewRequirements() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages approval requirements on an existing classic branch protection rule.",
		CreateContext: createBranchProtectionReviewRequirements,
		ReadContext:   readBranchProtectionReviewRequirements,
		UpdateContext: updateBranchProtectionReviewRequirements,
		DeleteContext: schema.NoopContext,
		Importer:      &schema.ResourceImporter{StateContext: importBranchProtectionReviewRequirements},
		Schema: map[string]*schema.Schema{
			"repository": {
				Type: schema.TypeString, Required: true, ForceNew: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			"pattern": {
				Type: schema.TypeString, Required: true, ForceNew: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			"required_approving_review_count": {
				Type: schema.TypeInt, Required: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.IntBetween(0, 6)),
			},
			"require_code_owner_reviews": {
				Type: schema.TypeBool, Required: true,
			},
			"require_last_push_approval": {
				Type: schema.TypeBool, Required: true,
			},
		},
	}
}

func branchProtectionReviewRequirementsID(repository, pattern string, meta any) (string, error) {
	repoID, err := getRepositoryID(repository, meta)
	if err != nil {
		return "", err
	}
	ruleID, err := getBranchProtectionID(repoID, pattern, meta)
	if err != nil {
		return "", err
	}
	return ruleID.(string), nil
}

func createBranchProtectionReviewRequirements(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	id, err := branchProtectionReviewRequirementsID(d.Get("repository").(string), d.Get("pattern").(string), meta)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(id)
	return updateBranchProtectionReviewRequirements(ctx, d, meta)
}

type branchProtectionReviewRequirements struct {
	ID                           githubv4.String
	Pattern                      githubv4.String
	RequiresApprovingReviews     githubv4.Boolean
	RequiredApprovingReviewCount githubv4.Int
	RequiresCodeOwnerReviews     githubv4.Boolean
	RequireLastPushApproval      githubv4.Boolean
	Repository                   struct {
		Name githubv4.String
	}
}

func readBranchProtectionReviewRequirements(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var query struct {
		Node struct {
			Rule branchProtectionReviewRequirements `graphql:"... on BranchProtectionRule"`
		} `graphql:"node(id: $id)"`
	}
	client := meta.(*Owner).v4client
	if err := client.Query(ctx, &query, map[string]any{"id": githubv4.ID(d.Id())}); err != nil {
		if strings.Contains(err.Error(), "Could not resolve to a node with the global id") {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	if query.Node.Rule.ID == "" {
		d.SetId("")
		return nil
	}
	return setBranchProtectionReviewRequirements(d, query.Node.Rule)
}

func setBranchProtectionReviewRequirements(d *schema.ResourceData, rule branchProtectionReviewRequirements) diag.Diagnostics {
	if !rule.RequiresApprovingReviews {
		return diag.Errorf("branch protection rule %s must already require a pull request", d.Id())
	}
	for key, value := range map[string]any{
		"repository":                      string(rule.Repository.Name),
		"pattern":                         string(rule.Pattern),
		"required_approving_review_count": int(rule.RequiredApprovingReviewCount),
		"require_code_owner_reviews":      bool(rule.RequiresCodeOwnerReviews),
		"require_last_push_approval":      bool(rule.RequireLastPushApproval),
	} {
		if err := d.Set(key, value); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func updateBranchProtectionReviewRequirements(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var mutation struct {
		UpdateBranchProtectionRule struct {
			BranchProtectionRule branchProtectionReviewRequirements
		} `graphql:"updateBranchProtectionRule(input: $input)"`
	}
	// Omitting all other input fields preserves the PR gate, app-bound checks,
	// push restrictions, and other settings on the existing rule.
	input := githubv4.UpdateBranchProtectionRuleInput{
		BranchProtectionRuleID:       d.Id(),
		RequiredApprovingReviewCount: new(githubv4.Int(d.Get("required_approving_review_count").(int))),
		RequiresCodeOwnerReviews:     new(githubv4.Boolean(d.Get("require_code_owner_reviews").(bool))),
		RequireLastPushApproval:      new(githubv4.Boolean(d.Get("require_last_push_approval").(bool))),
	}
	client := meta.(*Owner).v4client
	if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
		return diag.FromErr(err)
	}
	return setBranchProtectionReviewRequirements(d, mutation.UpdateBranchProtectionRule.BranchProtectionRule)
}

func importBranchProtectionReviewRequirements(_ context.Context, d *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
	repository, pattern, err := parseID2(d.Id())
	if err != nil {
		return nil, err
	}
	id, err := branchProtectionReviewRequirementsID(repository, pattern, meta)
	if err != nil {
		return nil, err
	}
	d.SetId(id)
	return []*schema.ResourceData{d}, nil
}
