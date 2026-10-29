package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestBusinessTeamRulesetBypass(t *testing.T) {
	for name, resource := range map[string]*schema.Resource{
		"organization": resourceGithubOrganizationRuleset(),
		"repository":   resourceGithubRepositoryRuleset(),
	} {
		t.Run(name, func(t *testing.T) {
			actorSchema := resource.Schema["bypass_actors"].Elem.(*schema.Resource).Schema["actor_type"]
			if diagnostics := actorSchema.ValidateDiagFunc("BusinessTeam", cty.GetAttrPath("actor_type")); diagnostics.HasError() {
				t.Fatalf("BusinessTeam rejected: %v", diagnostics)
			}
			actors := expandBypassActors([]any{map[string]any{
				"actor_id": 12345, "actor_type": "BusinessTeam", "bypass_mode": "pull_request",
			}})
			payload, err := json.Marshal(actors)
			if err != nil {
				t.Fatal(err)
			}
			var wire []map[string]any
			if err := json.Unmarshal(payload, &wire); err != nil {
				t.Fatal(err)
			}
			if wire[0]["actor_type"] != "BusinessTeam" || wire[0]["actor_id"] != float64(12345) {
				t.Fatalf("Incorrect API payload: %s", payload)
			}
			refreshed := flattenBypassActors(context.Background(), actors)[0].(map[string]any)
			refreshedJSON, err := json.Marshal(refreshed)
			if err != nil {
				t.Fatal(err)
			}
			var refreshedWire map[string]any
			if err := json.Unmarshal(refreshedJSON, &refreshedWire); err != nil {
				t.Fatal(err)
			}
			if refreshedWire["actor_type"] != "BusinessTeam" {
				t.Fatalf("Refresh lost actor type: %v", refreshed)
			}
		})
	}
}
