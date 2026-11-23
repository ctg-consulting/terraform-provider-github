package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	gh "github.com/google/go-github/v92/github"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestOrganizationActionsEventPolicyLifecycle(t *testing.T) {
	var policy actionsEventPolicy
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/orgs/example/actions/policies"
		if r.Method != http.MethodPost {
			expected += "/42"
		}
		if r.URL.Path != expected || r.Header.Get("X-GitHub-Api-Version") != "2026-03-10" ||
			r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("unexpected Actions policy endpoint or request headers")
		}
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var scope gh.RepositoryRulesetRepositoryPropertyConditionParameters
			if err := json.Unmarshal(policy.Conditions["repository_property"], &scope); err != nil {
				t.Error(err)
			}
			if len(scope.Include) != 1 || scope.Include[0].Name != "visibility" ||
				scope.Include[0].Source == nil || *scope.Include[0].Source != "system" ||
				!slices.Equal(scope.Include[0].PropertyValues, []string{"public"}) {
				t.Error("policy must target public repositories")
			}
			if len(policy.Rules) != 1 || policy.Rules[0].Type != "restrict_action_events" ||
				!slices.Contains(policy.Rules[0].Parameters.AllowedEvents, "pull_request") ||
				slices.Contains(policy.Rules[0].Parameters.AllowedEvents, "pull_request_target") {
				t.Error("unexpected Actions event rule")
			}
			policy.ID = 42
		case http.MethodDelete:
			policy = actionsEventPolicy{}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if policy.ID == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewEncoder(w).Encode(policy); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	api := mustCreateTestGitHubClient(t, server.URL+"/", gh.WithHTTPClient(server.Client()), gh.WithAuthToken("fixture-token"))
	owner := &Owner{name: "example", v3client: api}
	resource := resourceGithubOrganizationActionsEventPolicy()
	d := schema.TestResourceDataRaw(t, resource.Schema, map[string]any{
		"name": "Public workflow events", "enforcement": "active",
		"repository_visibility": "public", "allowed_events": []any{"pull_request", "push"},
	})
	ctx := context.Background()
	if ds := resource.CreateContext(ctx, d, owner); ds.HasError() || d.Id() != "42" {
		t.Fatalf("create: ID %q, diagnostics %v", d.Id(), ds)
	}
	if err := d.Set("enforcement", "evaluate"); err != nil {
		t.Fatal(err)
	}
	if ds := resource.UpdateContext(ctx, d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	imported := schema.TestResourceDataRaw(t, resource.Schema, nil)
	imported.SetId("42")
	if _, err := resource.Importer.StateContext(ctx, imported, owner); err != nil {
		t.Fatal(err)
	}
	if ds := resource.ReadContext(ctx, imported, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if imported.Get("enforcement") != "evaluate" || imported.Get("repository_visibility") != "public" {
		t.Fatalf("unexpected imported state: %v", imported.State())
	}
	if ds := resource.DeleteContext(ctx, d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if ds := resource.ReadContext(ctx, imported, owner); ds.HasError() || imported.Id() != "" {
		t.Fatalf("deleted policy remained in state: ID %q, diagnostics %v", imported.Id(), ds)
	}
}
