/*
Copyright 2024 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package route

import (
	"context"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	compapi "github.com/nholuongut/dapr/pkg/apis/components/v1alpha1"
	subapi "github.com/nholuongut/dapr/pkg/apis/subscriptions/v1alpha1"
	rtv1 "github.com/nholuongut/dapr/pkg/proto/runtime/v1"
	"github.com/nholuongut/dapr/tests/integration/framework"
	"github.com/nholuongut/dapr/tests/integration/framework/process/daprd"
	"github.com/nholuongut/dapr/tests/integration/framework/process/exec"
	"github.com/nholuongut/dapr/tests/integration/framework/process/grpc/subscriber"
	"github.com/nholuongut/dapr/tests/integration/framework/process/kubernetes"
	"github.com/nholuongut/dapr/tests/integration/framework/process/operator"
	"github.com/nholuongut/dapr/tests/integration/framework/process/sentry"
	"github.com/nholuongut/dapr/tests/integration/suite"
)

func init() {
	suite.Register(new(grpc))
}

type grpc struct {
	daprd    *daprd.Daprd
	kubeapi  *kubernetes.Kubernetes
	operator *operator.Operator
	sub      *subscriber.Subscriber
}

func (g *grpc) Setup(t *testing.T) []framework.Option {
	sentry := sentry.New(t, sentry.WithTrustDomain("integration.test.dapr.io"))
	g.sub = subscriber.New(t)

	g.kubeapi = kubernetes.New(t,
		kubernetes.WithBaseOperatorAPI(t,
			spiffeid.RequireTrustDomainFromString("integration.test.dapr.io"),
			"default",
			sentry.Port(),
		),
		kubernetes.WithClusterDaprComponentList(t, &compapi.ComponentList{
			Items: []compapi.Component{{
				ObjectMeta: metav1.ObjectMeta{Name: "mypub", Namespace: "default"},
				Spec: compapi.ComponentSpec{
					Type: "pubsub.in-memory", Version: "v1",
				},
			}},
		}),
		kubernetes.WithClusterDaprSubscriptionList(t, &subapi.SubscriptionList{
			Items: []subapi.Subscription{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mysub1", Namespace: "default"},
					Spec: subapi.SubscriptionSpec{
						Pubsubname: "mypub",
						Topic:      "a",
						Route:      "/a/b/c/d",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mysub2", Namespace: "default"},
					Spec: subapi.SubscriptionSpec{
						Pubsubname: "mypub",
						Topic:      "a",
						Route:      "/a",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mysub3", Namespace: "default"},
					Spec: subapi.SubscriptionSpec{
						Pubsubname: "mypub",
						Topic:      "b",
						Route:      "/a",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mysub4", Namespace: "default"},
					Spec: subapi.SubscriptionSpec{
						Pubsubname: "mypub",
						Topic:      "b",
						Route:      "/d/c/b/a",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mysub5", Namespace: "default"},
					Spec: subapi.SubscriptionSpec{
						Pubsubname: "mypub",
						Topic:      "b",
						Route:      "/a/b/c/d",
					},
				},
			},
		}),
	)

	g.operator = operator.New(t,
		operator.WithNamespace("default"),
		operator.WithKubeconfigPath(g.kubeapi.KubeconfigPath(t)),
		operator.WithTrustAnchorsFile(sentry.TrustAnchorsFile(t)),
	)

	g.daprd = daprd.New(t,
		daprd.WithMode("kubernetes"),
		daprd.WithSentryAddress(sentry.Address()),
		daprd.WithControlPlaneAddress(g.operator.Address()),
		daprd.WithAppPort(g.sub.Port(t)),
		daprd.WithAppProtocol("grpc"),
		daprd.WithDisableK8sSecretStore(true),
		daprd.WithEnableMTLS(true),
		daprd.WithNamespace("default"),
		daprd.WithControlPlaneTrustDomain("integration.test.dapr.io"),
		daprd.WithExecOptions(exec.WithEnvVars(t,
			"DAPR_TRUST_ANCHORS", string(sentry.CABundle().TrustAnchors),
		)),
	)

	return []framework.Option{
		framework.WithProcesses(sentry, g.sub, g.kubeapi, g.operator, g.daprd),
	}
}

func (g *grpc) Run(t *testing.T, ctx context.Context) {
	g.operator.WaitUntilRunning(t, ctx)
	g.daprd.WaitUntilRunning(t, ctx)

	client := g.daprd.GRPCClient(t, ctx)

	meta, err := client.GetMetadata(ctx, new(rtv1.GetMetadataRequest))
	require.NoError(t, err)
	assert.Len(t, meta.GetSubscriptions(), 2)

	_, err = client.PublishEvent(ctx, &rtv1.PublishEventRequest{
		PubsubName: "mypub",
		Topic:      "a",
	})
	require.NoError(t, err)
	resp := g.sub.Receive(t, ctx)
	assert.Contains(t, []string{"/a", "/a/b/c/d"}, resp.GetPath())
	assert.Empty(t, resp.GetData())

	_, err = client.PublishEvent(ctx, &rtv1.PublishEventRequest{
		PubsubName: "mypub",
		Topic:      "b",
	})
	require.NoError(t, err)
	resp = g.sub.Receive(t, ctx)
	assert.Contains(t, []string{"/a", "/a/b/c/d", "/d/c/b/a"}, resp.GetPath())
	assert.Empty(t, resp.GetData())
}
