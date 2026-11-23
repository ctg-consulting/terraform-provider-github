package github

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestBranchProtectionReviewRequirementsUpdatesOnlyApprovalFields(t *testing.T) {
	approvalCount := 2
	codeOwners := true
	lastPush := true
	mutations := 0
	owner := &Owner{v4client: newTestGraphQLClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		isUpdate := strings.Contains(request.Query, "updateBranchProtectionRule")
		if isUpdate {
			mutations++
			expected := map[string]any{
				"branchProtectionRuleId":       "BPR_example",
				"requiredApprovingReviewCount": float64(0),
				"requiresCodeOwnerReviews":     false,
				"requireLastPushApproval":      false,
			}
			if !reflect.DeepEqual(request.Variables["input"], expected) {
				t.Errorf("unexpected update input: %v", request.Variables["input"])
			}
			approvalCount, codeOwners, lastPush = 0, false, false
		}
		rule := map[string]any{
			"id":                           "BPR_example",
			"pattern":                      "release/*",
			"repository":                   map[string]any{"name": "example"},
			"requiresApprovingReviews":     true,
			"requiredApprovingReviewCount": approvalCount,
			"requiresCodeOwnerReviews":     codeOwners,
			"requireLastPushApproval":      lastPush,
		}
		response := map[string]any{"node": rule}
		if isUpdate {
			response = map[string]any{"updateBranchProtectionRule": map[string]any{"branchProtectionRule": rule}}
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"data": response}); err != nil {
			t.Error(err)
		}
	}))}
	resource := resourceGithubBranchProtectionReviewRequirements()
	data := schema.TestResourceDataRaw(t, resource.Schema, map[string]any{
		"repository":                      "example",
		"pattern":                         "release/*",
		"required_approving_review_count": 0,
		"require_code_owner_reviews":      false,
		"require_last_push_approval":      false,
	})
	data.SetId("BPR_example")
	ctx := context.Background()
	if ds := resource.UpdateContext(ctx, data, owner); ds.HasError() || data.Get("required_approving_review_count") != 0 {
		t.Fatalf("update: %v, state: %v", ds, data.State())
	}

	approvalCount, codeOwners, lastPush = 2, true, true
	if ds := resource.ReadContext(ctx, data, owner); ds.HasError() ||
		data.Get("required_approving_review_count") != 2 ||
		data.Get("require_code_owner_reviews") != true || data.Get("require_last_push_approval") != true {
		t.Fatalf("read: %v, state: %v", ds, data.State())
	}
	if ds := resource.DeleteContext(ctx, data, owner); ds.HasError() || mutations != 1 {
		t.Fatalf("delete: %v, saw %d mutations", ds, mutations)
	}
}
