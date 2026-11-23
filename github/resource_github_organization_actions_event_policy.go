package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/go-github/v92/github"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceGithubOrganizationActionsEventPolicy() *schema.Resource {
	return &schema.Resource{
		Description:   "Restricts Actions events for repositories selected by visibility.",
		CreateContext: writeOrganizationActionsEventPolicy,
		ReadContext:   readOrganizationActionsEventPolicy,
		UpdateContext: writeOrganizationActionsEventPolicy,
		DeleteContext: deleteOrganizationActionsEventPolicy,
		Importer:      &schema.ResourceImporter{StateContext: schema.ImportStatePassthroughContext},
		Schema: map[string]*schema.Schema{
			"name": {
				Type: schema.TypeString, Required: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			"enforcement": {
				Type: schema.TypeString, Required: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"active", "disabled", "evaluate"}, false)),
			},
			"repository_visibility": {
				Type: schema.TypeString, Required: true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"public", "private", "internal"}, false)),
			},
			"allowed_events": {
				Type: schema.TypeSet, Required: true, MinItems: 1,
				Elem: &schema.Schema{Type: schema.TypeString, ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace)},
			},
		},
	}
}

type actionsEventPolicy struct {
	ID          int64                      `json:"id,omitempty"`
	Name        string                     `json:"name"`
	Enforcement string                     `json:"enforcement"`
	Conditions  map[string]json.RawMessage `json:"conditions"`
	Rules       []actionsEventPolicyRule   `json:"rules"`
}

type actionsEventPolicyRule struct {
	Type       string `json:"type"`
	Parameters struct {
		AllowedEvents []string `json:"allowed_events"`
	} `json:"parameters"`
}

func organizationActionsEventPolicyRequest(ctx context.Context, owner *Owner, method, id string, body, result any) (*github.Response, error) {
	path := fmt.Sprintf("orgs/%s/actions/policies", owner.name)
	if id != "" {
		value, err := strconv.ParseInt(id, 10, 64)
		if err != nil || value <= 0 {
			return nil, fmt.Errorf("expected a positive organization Actions policy ID")
		}
		path += "/" + id
	}
	req, err := owner.v3client.NewRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	return owner.v3client.Do(req, result)
}

func writeOrganizationActionsEventPolicy(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	scope, err := json.Marshal(github.RepositoryRulesetRepositoryPropertyConditionParameters{
		Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{{
			Name: "visibility", Source: new("system"), PropertyValues: []string{d.Get("repository_visibility").(string)},
		}},
		Exclude: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{},
	})
	if err != nil {
		return diag.FromErr(err)
	}
	rule := actionsEventPolicyRule{Type: "restrict_action_events"}
	for _, event := range d.Get("allowed_events").(*schema.Set).List() {
		rule.Parameters.AllowedEvents = append(rule.Parameters.AllowedEvents, event.(string))
	}
	request := actionsEventPolicy{
		Name: d.Get("name").(string), Enforcement: d.Get("enforcement").(string),
		Conditions: map[string]json.RawMessage{"repository_property": scope},
		Rules:      []actionsEventPolicyRule{rule},
	}
	method := http.MethodPost
	if d.Id() != "" {
		method = http.MethodPut
	}
	var policy actionsEventPolicy
	if _, err := organizationActionsEventPolicyRequest(ctx, meta.(*Owner), method, d.Id(), request, &policy); err != nil {
		return diag.FromErr(err)
	}
	if policy.ID <= 0 {
		return diag.Errorf("GitHub returned no organization Actions policy ID")
	}
	d.SetId(strconv.FormatInt(policy.ID, 10))
	return readOrganizationActionsEventPolicy(ctx, d, meta)
}

func readOrganizationActionsEventPolicy(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var policy actionsEventPolicy
	resp, err := organizationActionsEventPolicyRequest(ctx, meta.(*Owner), http.MethodGet, d.Id(), nil, &policy)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound && !d.IsNewResource() {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	var scope github.RepositoryRulesetRepositoryPropertyConditionParameters
	if len(policy.Conditions) != 1 || json.Unmarshal(policy.Conditions["repository_property"], &scope) != nil ||
		len(scope.Exclude) != 0 || len(scope.Include) != 1 || scope.Include[0] == nil ||
		scope.Include[0].Name != "visibility" || scope.Include[0].Source == nil || *scope.Include[0].Source != "system" ||
		len(scope.Include[0].PropertyValues) != 1 || len(policy.Rules) != 1 ||
		policy.Rules[0].Type != "restrict_action_events" || len(policy.Rules[0].Parameters.AllowedEvents) == 0 {
		return diag.Errorf("organization Actions policy %s is not a standalone event allowlist scoped by repository visibility", d.Id())
	}
	for key, value := range map[string]any{
		"name": policy.Name, "enforcement": policy.Enforcement,
		"repository_visibility": scope.Include[0].PropertyValues[0],
		"allowed_events":        policy.Rules[0].Parameters.AllowedEvents,
	} {
		if err := d.Set(key, value); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func deleteOrganizationActionsEventPolicy(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	resp, err := organizationActionsEventPolicyRequest(ctx, meta.(*Owner), http.MethodDelete, d.Id(), nil, nil)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return diag.FromErr(err)
}
