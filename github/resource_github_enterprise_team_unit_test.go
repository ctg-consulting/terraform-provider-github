package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	gh "github.com/google/go-github/v92/github"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func enterpriseTeamFixture(t *testing.T, handler http.HandlerFunc) *Owner {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authenticated transport was not preserved")
		}
		if r.Header.Get("X-GitHub-Api-Version") != "2026-03-10" {
			t.Errorf("incorrect enterprise API version")
		}
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	api := mustCreateTestGitHubClient(t, server.URL+"/", gh.WithHTTPClient(server.Client()), gh.WithAuthToken("fixture-token"))
	return &Owner{v3client: api}
}

func teamFixtureJSON(mode string) string {
	return fmt.Sprintf(`{"id":42,"name":"Platform","slug":"ent:platform","description":"","group_id":null,"organization_selection_type":%q,"notification_setting":"notifications_enabled"}`, mode)
}

func TestEnterpriseTeamResourceImports(t *testing.T) {
	for name, resource := range map[string]*schema.Resource{
		"team":          resourceGithubEnterpriseTeam(),
		"membership":    resourceGithubEnterpriseTeamMembership(),
		"organizations": resourceGithubEnterpriseTeamOrganizations(),
	} {
		t.Run(name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, resource.Schema, nil)
			id := "example/42"
			if name == "membership" {
				id += "/octocat"
			}
			d.SetId(id)
			if _, err := resource.Importer.StateContext(context.Background(), d, nil); err != nil {
				t.Fatal(err)
			}
			if d.Id() != id || d.Get("enterprise_slug") != "example" || d.Get("team_id") != 42 {
				t.Fatalf("incorrect imported identity: %v", d.State())
			}
			d.SetId("example/not-an-id")
			if _, err := resource.Importer.StateContext(context.Background(), d, nil); err == nil {
				t.Fatal("invalid import accepted")
			}
		})
	}
}

func TestEnterpriseTeamReadPreservesStateOnErrors(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusOK} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			owner := enterpriseTeamFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"id":42,"slug":"ent:platform","name":"Changed"}`)
			})
			resource := resourceGithubEnterpriseTeam()
			d := schema.TestResourceDataRaw(t, resource.Schema, map[string]any{"enterprise_slug": "example", "team_id": 42, "slug": "ent:platform", "name": "Original"})
			d.SetId("example/42")
			if ds := resource.ReadContext(context.Background(), d, owner); !ds.HasError() {
				t.Fatal("expected incomplete response or HTTP error")
			}
			if d.Id() != "example/42" || d.Get("name") != "Original" {
				t.Fatal("read failure altered existing state")
			}
		})
	}
}

func TestEnterpriseTeamRefreshAndMembershipLifecycle(t *testing.T) {
	var membershipWrites []string
	owner := enterpriseTeamFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/enterprises/example/teams":
			fmt.Fprintf(w, "[%s]", teamFixtureJSON("selected"))
		case "/enterprises/example/teams/ent:platform":
			fmt.Fprint(w, teamFixtureJSON("selected"))
		case "/enterprises/example/teams/42/memberships/octocat":
			if r.Method != "GET" {
				membershipWrites = append(membershipWrites, r.Method)
			}
			if r.Method == "DELETE" {
				w.WriteHeader(http.StatusNoContent)
			} else {
				fmt.Fprint(w, `{"id":99,"login":"octocat"}`)
			}
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	team := resourceGithubEnterpriseTeam()
	d := schema.TestResourceDataRaw(t, team.Schema, map[string]any{"enterprise_slug": "example", "team_id": 42})
	d.SetId("example/42")
	if ds := team.ReadContext(context.Background(), d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if d.Get("slug") != "ent:platform" || d.Get("organization_selection_type") != "selected" {
		t.Fatal("team not refreshed")
	}
	membership := resourceGithubEnterpriseTeamMembership()
	d = schema.TestResourceDataRaw(t, membership.Schema, map[string]any{"enterprise_slug": "example", "team_id": 42, "username": "octocat"})
	if ds := membership.CreateContext(context.Background(), d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if d.Id() != "example/42/octocat" || d.Get("user_id") != 99 {
		t.Fatal("incorrect membership identity")
	}
	if ds := membership.ReadContext(context.Background(), d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if ds := membership.DeleteContext(context.Background(), d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if d.Id() != "" || !reflect.DeepEqual(membershipWrites, []string{"PUT", "DELETE"}) {
		t.Fatalf("incorrect membership operations: %v", membershipWrites)
	}
}

func TestEnterpriseTeamOrganizationsReconcileAndGuard(t *testing.T) {
	for _, mode := range []string{"selected", "all"} {
		t.Run(mode, func(t *testing.T) {
			current := map[string]bool{"old-org": true}
			var writes []string
			owner := enterpriseTeamFixture(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/enterprises/example/teams":
					fmt.Fprintf(w, "[%s]", teamFixtureJSON(mode))
				case "/enterprises/example/teams/ent:platform":
					fmt.Fprint(w, teamFixtureJSON(mode))
				case "/enterprises/example/teams/42/organizations":
					result := []map[string]string{}
					for name := range current {
						result = append(result, map[string]string{"login": name})
					}
					_ = json.NewEncoder(w).Encode(result)
				default:
					if !strings.HasPrefix(r.URL.Path, "/enterprises/example/teams/42/organizations/") || r.Method != "POST" {
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
						w.WriteHeader(404)
						return
					}
					var body struct {
						Organizations []string `json:"organization_slugs"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					op := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
					writes = append(writes, op)
					for _, name := range body.Organizations {
						if op == "add" {
							current[name] = true
						} else {
							delete(current, name)
						}
					}
					w.WriteHeader(http.StatusNoContent)
				}
			})
			resource := resourceGithubEnterpriseTeamOrganizations()
			d := schema.TestResourceDataRaw(t, resource.Schema, map[string]any{"enterprise_slug": "example", "team_id": 42, "organization_slugs": []any{"new-org"}})
			// Creation must never overwrite an existing assignment set.
			if ds := resource.CreateContext(context.Background(), d, owner); !ds.HasError() {
				t.Fatal("creation should require import")
			}
			if len(writes) != 0 {
				t.Fatal("creation mutated existing assignments")
			}
			d.SetId("example/42")
			ds := resource.UpdateContext(context.Background(), d, owner)
			if mode == "all" {
				if !ds.HasError() || len(writes) != 0 || d.Id() != "example/42" {
					t.Fatal("all-organizations mode was not protected")
				}
				return
			}
			if ds.HasError() {
				t.Fatal(ds)
			}
			if !reflect.DeepEqual(writes, []string{"add", "remove"}) || !current["new-org"] || current["old-org"] {
				t.Fatalf("incorrect assignment reconciliation: %v %v", writes, current)
			}
			if err := d.Set("organization_slugs", []string{}); err != nil {
				t.Fatal(err)
			}
			if ds := resource.UpdateContext(context.Background(), d, owner); ds.HasError() {
				t.Fatal(ds)
			}
			if len(current) != 0 {
				t.Fatal("empty configuration did not clear assignments")
			}
		})
	}
}
