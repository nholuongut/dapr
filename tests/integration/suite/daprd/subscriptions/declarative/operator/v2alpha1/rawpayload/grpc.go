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

package rawpayload

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dapr/components-contrib/pubsub"
	compapi "github.com/nholuongut/dapr/pkg/apis/components/v1alpha1"
	subapi "github.com/nholuongut/dapr/pkg/apis/subscriptions/v2alpha1"
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
		kubernetes.WithClusterDaprSubscriptionListV2(t, &subapi.SubscriptionList{
			Items: []subapi.Subscription{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mysub", Namespace: "default"},
					Spec: subapi.SubscriptionSpec{
						Pubsubname: "mypub",
						Topic:      "a",
						Routes: subapi.Routes{
							Default: "/a",
						},
						Metadata: map[string]string{
							"rawPayload": "true",
						},
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

	_, err := client.PublishEvent(ctx, &rtv1.PublishEventRequest{
		PubsubName: "mypub", Topic: "a", Data: []byte(`{"status": "completed"}`),
		Metadata: map[string]string{"foo": "bar"}, DataContentType: "application/json",
	})
	require.NoError(t, err)
	resp := g.sub.Receive(t, ctx)
	assert.Equal(t, "/a", resp.GetPath())
	assert.Equal(t, "1.0", resp.GetSpecVersion())
	assert.Equal(t, "mypub", resp.GetPubsubName())
	assert.Equal(t, "com.dapr.event.sent", resp.GetType())
	assert.Equal(t, "application/octet-stream", resp.GetDataContentType())
	var ce map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(resp.GetData())).Decode(&ce))
	exp := pubsub.NewCloudEventsEnvelope("", "", "com.dapr.event.sent", "", "a", "mypub", "application/json", []byte(`{"status": "completed"}`), "", "")
	exp["id"] = ce["id"]
	exp["source"] = ce["source"]
	exp["time"] = ce["time"]
	exp["traceid"] = ce["traceid"]
	exp["traceparent"] = ce["traceparent"]
	assert.Equal(t, exp, ce)

	_, err = client.PublishEvent(ctx, &rtv1.PublishEventRequest{
		PubsubName: "mypub", Topic: "a",
		Data: []byte(`{"status": "completed"}`), DataContentType: "foo/bar",
	})
	require.NoError(t, err)
	resp = g.sub.Receive(t, ctx)
	assert.Equal(t, "/a", resp.GetPath())
	assert.Equal(t, "1.0", resp.GetSpecVersion())
	assert.Equal(t, "mypub", resp.GetPubsubName())
	assert.Equal(t, "com.dapr.event.sent", resp.GetType())
	assert.Equal(t, "application/octet-stream", resp.GetDataContentType())
	require.NoError(t, json.NewDecoder(bytes.NewReader(resp.GetData())).Decode(&ce))
	exp = pubsub.NewCloudEventsEnvelope("", "", "com.dapr.event.sent", "", "a", "mypub", "application/json", nil, "", "")
	exp["id"] = ce["id"]
	exp["data"] = `{"status": "completed"}`
	exp["datacontenttype"] = "foo/bar"
	exp["source"] = ce["source"]
	exp["time"] = ce["time"]
	exp["traceid"] = ce["traceid"]
	exp["traceparent"] = ce["traceparent"]
	assert.Equal(t, exp, ce)

	_, err = client.PublishEvent(ctx, &rtv1.PublishEventRequest{
		PubsubName: "mypub", Topic: "a", Data: []byte("foo"),
	})
	require.NoError(t, err)
	resp = g.sub.Receive(t, ctx)
	assert.Equal(t, "/a", resp.GetPath())
	assert.Equal(t, "application/octet-stream", resp.GetDataContentType())
	assert.Equal(t, "1.0", resp.GetSpecVersion())
	assert.Equal(t, "mypub", resp.GetPubsubName())
	assert.Equal(t, "com.dapr.event.sent", resp.GetType())
	require.NoError(t, json.NewDecoder(bytes.NewReader(resp.GetData())).Decode(&ce))
	exp = pubsub.NewCloudEventsEnvelope("", "", "com.dapr.event.sent", "", "a", "mypub", "application/json", []byte("foo"), "", "")
	exp["id"] = ce["id"]
	exp["source"] = ce["source"]
	exp["time"] = ce["time"]
	exp["traceid"] = ce["traceid"]
	exp["traceparent"] = ce["traceparent"]
	exp["datacontenttype"] = "text/plain"
	assert.Equal(t, exp, ce)
}
