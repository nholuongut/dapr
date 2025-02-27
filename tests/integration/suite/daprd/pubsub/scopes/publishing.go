/*
Copyright 2024 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implieh.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scopes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rtv1 "github.com/nholuongut/dapr/pkg/proto/runtime/v1"
	"github.com/nholuongut/dapr/tests/integration/framework"
	"github.com/nholuongut/dapr/tests/integration/framework/process/daprd"
	"github.com/nholuongut/dapr/tests/integration/framework/process/grpc/subscriber"
	"github.com/nholuongut/dapr/tests/integration/suite"
)

func init() {
	suite.Register(new(publishing))
}

type publishing struct {
	daprd1 *daprd.Daprd
	daprd2 *daprd.Daprd
	daprd3 *daprd.Daprd
	sub    *subscriber.Subscriber
}

func (p *publishing) Setup(t *testing.T) []framework.Option {
	p.sub = subscriber.New(t)

	resDir := t.TempDir()

	p.daprd1 = daprd.New(t,
		daprd.WithAppPort(p.sub.Port(t)),
		daprd.WithAppProtocol("grpc"),
		daprd.WithResourcesDir(resDir),
	)
	p.daprd2 = daprd.New(t,
		daprd.WithAppPort(p.sub.Port(t)),
		daprd.WithAppProtocol("grpc"),
		daprd.WithResourcesDir(resDir),
	)
	p.daprd3 = daprd.New(t,
		daprd.WithAppPort(p.sub.Port(t)),
		daprd.WithAppProtocol("grpc"),
		daprd.WithResourcesDir(resDir),
	)

	var subYaml string
	for i, sub := range []struct {
		pubsub string
		topic  string
	}{
		{"all", "topic1"},
		{"allempty", "topic2"},
		{"app1-topic34", "topic3"},
		{"app1-topic34", "topic4"},
		{"app2-topic56", "topic5"},
		{"app2-topic56", "topic6"},
		{"app1-topic7-app2-topic8", "topic7"},
		{"app1-topic7-app2-topic8", "topic8"},
		{"app1-nil-app2-topic910", "topic9"},
		{"app1-nil-app2-topic910", "topic10"},
	} {
		subYaml += fmt.Sprintf(`
---
apiVersion: dapr.io/v1alpha1
kind: Subscription
metadata:
 name: sub%d
spec:
 pubsubname: %s
 topic: %s
 route: /a
`, i+1, sub.pubsub, sub.topic)
	}
	require.NoError(t, os.WriteFile(filepath.Join(resDir, "sub.yaml"), []byte(subYaml), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(resDir, "pubsub.yaml"),
		[]byte(fmt.Sprintf(`
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: all
spec:
 type: pubsub.in-memory
 version: v1
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: allempty
spec:
 type: pubsub.in-memory
 version: v1
 metadata:
 - name: publishingScopes
   value: ""
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: app1-topic34
spec:
 type: pubsub.in-memory
 version: v1
 metadata:
 - name: publishingScopes
   value: "%[1]s=topic3,topic4"
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: app2-topic56
spec:
 type: pubsub.in-memory
 version: v1
 metadata:
 - name: publishingScopes
   value: "%[2]s=topic5,topic6"
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: app1-topic7-app2-topic8
spec:
 type: pubsub.in-memory
 version: v1
 metadata:
 - name: publishingScopes
   value: "%[1]s=topic7;%[2]s=topic8"
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: app1-nil-app2-topic910
spec:
 type: pubsub.in-memory
 version: v1
 metadata:
 - name: publishingScopes
   value: "%[1]s=;%[2]s=topic9,topic10"
`, p.daprd1.AppID(), p.daprd2.AppID())), 0o600))

	return []framework.Option{
		framework.WithProcesses(p.sub, p.daprd1, p.daprd2, p.daprd3),
	}
}

func (p *publishing) Run(t *testing.T, ctx context.Context) {
	p.daprd1.WaitUntilRunning(t, ctx)
	p.daprd2.WaitUntilRunning(t, ctx)
	p.daprd3.WaitUntilRunning(t, ctx)

	for _, daprd := range []*daprd.Daprd{p.daprd1, p.daprd2, p.daprd3} {
		meta, err := daprd.GRPCClient(t, ctx).GetMetadata(ctx, new(rtv1.GetMetadataRequest))
		require.NoError(t, err)
		assert.Len(t, meta.GetRegisteredComponents(), 6)
		assert.Len(t, meta.GetSubscriptions(), 10)
	}

	newReq := func(pubsub, topic string) *rtv1.PublishEventRequest {
		return &rtv1.PublishEventRequest{PubsubName: pubsub, Topic: topic, Data: []byte(`{"status": "completed"}`)}
	}

	for _, req := range []*rtv1.PublishEventRequest{
		newReq("all", "topic1"),
		newReq("allempty", "topic2"),
		newReq("app1-topic34", "topic3"),
		newReq("app1-topic34", "topic4"),
		newReq("app2-topic56", "topic5"),
		newReq("app2-topic56", "topic6"),
	} {
		t.Run("pubsub="+req.GetPubsubName()+",topic="+req.GetTopic(), func(t *testing.T) {
			p.sub.ExpectPublishReceive(t, ctx, p.daprd1, req)
			p.sub.ExpectPublishReceive(t, ctx, p.daprd2, req)
			p.sub.ExpectPublishReceive(t, ctx, p.daprd3, req)
		})
	}

	req := newReq("app1-topic7-app2-topic8", "topic7")
	p.sub.ExpectPublishReceive(t, ctx, p.daprd1, req)
	p.sub.ExpectPublishError(t, ctx, p.daprd2, req)
	p.sub.ExpectPublishReceive(t, ctx, p.daprd3, req)

	req = newReq("app1-topic7-app2-topic8", "topic8")
	p.sub.ExpectPublishError(t, ctx, p.daprd1, req)
	p.sub.ExpectPublishReceive(t, ctx, p.daprd2, req)
	p.sub.ExpectPublishReceive(t, ctx, p.daprd3, req)

	req = newReq("app1-nil-app2-topic910", "topic9")
	p.sub.ExpectPublishError(t, ctx, p.daprd1, req)
	p.sub.ExpectPublishReceive(t, ctx, p.daprd2, req)
	p.sub.ExpectPublishReceive(t, ctx, p.daprd3, req)

	req = newReq("app1-nil-app2-topic910", "topic10")
	p.sub.ExpectPublishError(t, ctx, p.daprd1, req)
	p.sub.ExpectPublishReceive(t, ctx, p.daprd2, req)
	p.sub.ExpectPublishReceive(t, ctx, p.daprd3, req)
}
