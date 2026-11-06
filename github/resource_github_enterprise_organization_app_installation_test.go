package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestEnterpriseOrganizationAppInstallationLifecycle(t *testing.T) {
	installed := false
	owner := enterpriseTeamFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/enterprises/example/apps/organizations/octo/installations" &&
			r.URL.Path != "/enterprises/example/apps/organizations/octo/installations/42" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"client_id":"Iv23example"`) || !strings.Contains(string(body), `"repository_selection":"all"`) {
				t.Fatalf("wrong install request %s", body)
			}
			installed = true
			fmt.Fprint(w, `{"id":42,"app_slug":"ctg-elt","repository_selection":"all"}`)
		case http.MethodGet:
			if installed {
				fmt.Fprint(w, `[{"value":{"id":42,"app_slug":"ctg-elt","repository_selection":"all"}}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
		case http.MethodDelete:
			if !installed {
				t.Fatal("delete before install")
			}
			installed = false
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	resource := resourceGithubEnterpriseOrganizationAppInstallation()
	d := schema.TestResourceDataRaw(t, resource.Schema, map[string]any{
		"enterprise_slug": "example", "organization": "octo", "client_id": "Iv23example", "repository_selection": "all",
	})
	if ds := resource.CreateContext(context.Background(), d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if d.Id() != "example/octo/42" || d.Get("app_slug") != "ctg-elt" {
		t.Fatalf("bad installation state: %v", d.State())
	}
	if ds := resource.DeleteContext(context.Background(), d, owner); ds.HasError() {
		t.Fatal(ds)
	}
	if d.Id() != "" || installed {
		t.Fatal("installation was not deleted")
	}
}

func TestEnterpriseOrganizationAppInstallationImport(t *testing.T) {
	r := resourceGithubEnterpriseOrganizationAppInstallation()
	d := schema.TestResourceDataRaw(t, r.Schema, nil)
	d.SetId("example/octo/42")
	if _, err := r.Importer.StateContext(context.Background(), d, nil); err != nil {
		t.Fatal(err)
	}
	if d.Get("enterprise_slug") != "example" || d.Get("organization") != "octo" || d.Get("installation_id") != 42 {
		t.Fatal("bad import state")
	}
	d.SetId("example/octo/nope")
	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("invalid import accepted")
	}
}
