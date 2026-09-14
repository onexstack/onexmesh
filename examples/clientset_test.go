// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package examples_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/onexstack/onexmesh/examples/apis/apps/v1"
	exampleclient "github.com/onexstack/onexmesh/examples/pkg/generated/exampleclient"
	applyconfigurationsappsv1 "github.com/onexstack/onexmesh/examples/pkg/generated/exampleclient/applyconfigurations/apps/v1"
	"github.com/onexstack/onexmesh/examples/pkg/generated/exampleclient/fake"
	"github.com/onexstack/onexmesh/pkg/client/rest"
	meshmeta "github.com/onexstack/onexmesh/pkg/proto/onexmesh/meta/v1"
	"github.com/onexstack/onexmesh/pkg/registry"
)

// staticDiscovery is an in-memory Discovery returning a fixed instance set.
type staticDiscovery struct {
	instances []*registry.ServiceInstance
}

func (d *staticDiscovery) GetService(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	return d.instances, nil
}

func (d *staticDiscovery) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	return nil, nil
}

func newDeployment(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ApiVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata:   &meshmeta.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       &appsv1.DeploymentSpec{Replicas: 3},
	}
}

// TestFakeClientsetCRUD exercises the in-memory fake clientset: Create/Get/List
// plus server-side Apply all go through the generated typed client.
func TestFakeClientsetCRUD(t *testing.T) {
	client := fake.NewSimpleClientset(newDeployment("d1", "default"))
	ctx := context.Background()

	got, err := client.AppsV1().Deployments("default").Get(ctx, "d1", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata.Name != "d1" || got.Spec.Replicas != 3 {
		t.Fatalf("got %+v", got)
	}

	list, err := client.AppsV1().Deployments("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("List returned %d items, want 1", len(list.Items))
	}

	cfg := applyconfigurationsappsv1.Deployment("d1", "default").
		WithSpec(&appsv1.DeploymentSpec{Replicas: 5})
	applied, err := client.AppsV1().Deployments("default").Apply(ctx, cfg, metav1.ApplyOptions{FieldManager: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Spec.Replicas != 5 {
		t.Fatalf("Apply result replicas = %d, want 5", applied.Spec.Replicas)
	}
}

// TestNewForMeshList drives the mesh-aware clientset end-to-end: discovery
// resolves the service, the meshRoundTripper rewrites the request to the
// selected instance, and the typed client decodes the response.
func TestNewForMeshList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"apps/v1","kind":"DeploymentList","metadata":{"resourceVersion":"1"},"items":[]}`))
	}))
	defer srv.Close()

	d := &staticDiscovery{instances: []*registry.ServiceInstance{{
		ID:        "i1",
		Name:      "edu.course.student-api",
		Endpoints: []string{"http://" + srv.Listener.Addr().String()},
	}}}

	client, err := exampleclient.NewForMesh("edu.course.student-api", rest.WithDiscovery(d))
	if err != nil {
		t.Fatal(err)
	}

	list, err := client.AppsV1().Deployments("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Fatal("expected a non-nil DeploymentList")
	}
}
