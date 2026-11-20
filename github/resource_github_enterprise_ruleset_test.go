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

func TestEnterpriseRulesetLifecycle(t *testing.T) {
	var stored gh.RepositoryRuleset
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing provider authentication")
		}
		path := "/enterprises/example/rulesets"
		if r.Method != http.MethodPost {
			path += "/42"
		}
		if r.URL.Path != path {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&stored); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			conditions := stored.Conditions
			if conditions == nil || conditions.OrganizationID == nil ||
				!slices.Equal(conditions.OrganizationID.OrganizationIDs, []int64{123}) ||
				conditions.RepositoryName == nil || !slices.Equal(conditions.RepositoryName.Include, []string{"infra"}) ||
				conditions.RefName == nil || !slices.Equal(conditions.RefName.Include, []string{"~DEFAULT_BRANCH"}) ||
				stored.Rules == nil || stored.Rules.PullRequest == nil || stored.Rules.PullRequest.RequiredApprovingReviewCount != 1 {
				t.Errorf("unexpected enterprise ruleset request: %+v", stored)
			}
			stored.ID = new(int64(42))
		case http.MethodDelete:
			stored = gh.RepositoryRuleset{}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if stored.GetID() == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewEncoder(w).Encode(stored); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	api := mustCreateTestGitHubClient(t, server.URL+"/", gh.WithHTTPClient(server.Client()), gh.WithAuthToken("fixture-token"))
	owner := &Owner{v3client: api}
	resource := resourceGithubEnterpriseRuleset()
	d := schema.TestResourceDataRaw(t, resource.Schema, map[string]any{
		"enterprise_slug": "example", "name": "Approvals", "enforcement": "active",
		"organization_ids": []any{123},
		"conditions": []any{map[string]any{
			"repository_name": []any{map[string]any{"include": []any{"infra"}, "exclude": []any{}}},
			"ref_name":        []any{map[string]any{"include": []any{"~DEFAULT_BRANCH"}, "exclude": []any{}}},
		}},
		"rules": []any{map[string]any{
			"pull_request": []any{map[string]any{"required_approving_review_count": 1}},
		}},
	})
	ctx := context.Background()
	if ds := resource.CreateContext(ctx, d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if d.Id() != "example/42" || d.Get("ruleset_id") != 42 {
		t.Fatalf("unexpected created identity: %s (%v)", d.Id(), d.Get("ruleset_id"))
	}
	if err := d.Set("enforcement", "evaluate"); err != nil {
		t.Fatal(err)
	}
	if ds := resource.UpdateContext(ctx, d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	imported := schema.TestResourceDataRaw(t, resource.Schema, nil)
	imported.SetId(d.Id())
	if _, err := resource.Importer.StateContext(ctx, imported, owner); err != nil {
		t.Fatal(err)
	}
	if ds := resource.ReadContext(ctx, imported, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if imported.Get("enterprise_slug") != "example" || imported.Get("enforcement") != "evaluate" {
		t.Fatalf("unexpected imported state: %v", imported.State())
	}
	if ds := resource.DeleteContext(ctx, d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if ds := resource.ReadContext(ctx, imported, owner); ds.HasError() || imported.Id() != "" {
		t.Fatalf("missing ruleset should clear state: %s %v", imported.Id(), ds)
	}
	if !slices.Equal(methods, []string{"POST", "GET", "PUT", "GET", "GET", "DELETE", "GET"}) {
		t.Fatalf("unexpected requests: %v", methods)
	}
}
