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

package subscriptions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rtv1 "github.com/nholuongut/dapr/pkg/proto/runtime/v1"
	"github.com/nholuongut/dapr/tests/integration/framework"
	"github.com/nholuongut/dapr/tests/integration/framework/process/daprd"
	"github.com/nholuongut/dapr/tests/integration/framework/process/grpc/subscriber"
	"github.com/nholuongut/dapr/tests/integration/suite"
)

func init() {
	suite.Register(new(grpc))
}

type grpc struct {
	daprd *daprd.Daprd
	sub   *subscriber.Subscriber

	resDir1 string
	resDir2 string
}

func (g *grpc) Setup(t *testing.T) []framework.Option {
	g.sub = subscriber.New(t)

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(`
apiVersion: dapr.io/v1alpha1
kind: Configuration
metadata:
  name: hotreloading
spec:
  features:
  - name: HotReload
    enabled: true`), 0o600))

	g.resDir1, g.resDir2 = t.TempDir(), t.TempDir()

	for i, dir := range []string{g.resDir1, g.resDir2} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "pubsub.yaml"), []byte(fmt.Sprintf(`
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: 'pubsub%d'
spec:
 type: pubsub.in-memory
 version: v1
`, i)), 0o600))
	}

	g.daprd = daprd.New(t,
		daprd.WithAppPort(g.sub.Port(t)),
		daprd.WithAppProtocol("grpc"),
		daprd.WithConfigs(configFile),
		daprd.WithResourcesDir(g.resDir1, g.resDir2),
	)

	return []framework.Option{
		framework.WithProcesses(g.sub, g.daprd),
	}
}

func (g *grpc) Run(t *testing.T, ctx context.Context) {
	g.daprd.WaitUntilRunning(t, ctx)

	assert.Len(t, g.daprd.GetMetaRegisteredComponents(t, ctx), 2)
	assert.Empty(t, g.daprd.GetMetaSubscriptions(t, ctx))

	newReq := func(pubsub, topic string) *rtv1.PublishEventRequest {
		return &rtv1.PublishEventRequest{PubsubName: pubsub, Topic: topic, Data: []byte(`{"status": "completed"}`)}
	}
	g.sub.ExpectPublishNoReceive(t, ctx, g.daprd, newReq("pubsub0", "a"))

	require.NoError(t, os.WriteFile(filepath.Join(g.resDir1, "sub.yaml"), []byte(`
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub1'
spec:
 pubsubname: 'pubsub0'
 topic: 'a'
 routes:
  default: '/a'
`), 0o600))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, g.daprd.GetMetaSubscriptions(t, ctx), 1)
	}, time.Second*5, time.Millisecond*10)
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub0", "a"))

	g.sub.ExpectPublishNoReceive(t, ctx, g.daprd, newReq("pubsub0", "b"))
	require.NoError(t, os.WriteFile(filepath.Join(g.resDir2, "sub.yaml"), []byte(`
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub2'
spec:
 pubsubname: 'pubsub0'
 topic: 'b'
 routes:
  default: '/b'
`), 0o600))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, g.daprd.GetMetaSubscriptions(t, ctx), 2)
	}, time.Second*5, time.Millisecond*10)
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub0", "b"))

	require.NoError(t, os.Remove(filepath.Join(g.resDir1, "sub.yaml")))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, g.daprd.GetMetaSubscriptions(t, ctx), 1)
	}, time.Second*5, time.Millisecond*10)
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub0", "b"))
	g.sub.ExpectPublishNoReceive(t, ctx, g.daprd, newReq("pubsub0", "a"))

	g.sub.ExpectPublishNoReceive(t, ctx, g.daprd, newReq("pubsub1", "c"))
	require.NoError(t, os.WriteFile(filepath.Join(g.resDir2, "sub.yaml"), []byte(`
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub2'
spec:
 pubsubname: 'pubsub0'
 topic: 'b'
 routes:
  default: '/b'
---
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub3'
spec:
 pubsubname: 'pubsub1'
 topic: c
 routes:
  default: '/c'
`), 0o600))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, g.daprd.GetMetaSubscriptions(t, ctx), 2)
	}, time.Second*5, time.Millisecond*10)
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub0", "b"))
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub1", "c"))

	require.NoError(t, os.WriteFile(filepath.Join(g.resDir2, "sub.yaml"), []byte(`
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub2'
spec:
 pubsubname: 'pubsub0'
 topic: 'd'
 routes:
  default: '/d'
---
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub3'
spec:
 pubsubname: 'pubsub1'
 topic: c
 routes:
  default: '/c'
`), 0o600))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := g.daprd.GRPCClient(t, ctx).GetMetadata(ctx, new(rtv1.GetMetadataRequest))
		require.NoError(t, err)
		if assert.Len(c, resp.GetSubscriptions(), 2) {
			assert.Equal(c, "c", resp.GetSubscriptions()[0].GetTopic())
			assert.Equal(c, "d", resp.GetSubscriptions()[1].GetTopic())
		}
	}, time.Second*5, time.Millisecond*10)
	g.sub.ExpectPublishNoReceive(t, ctx, g.daprd, newReq("pubsub0", "b"))
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub0", "d"))
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub1", "c"))

	require.NoError(t, os.WriteFile(filepath.Join(g.resDir2, "sub.yaml"), []byte(`
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub2'
spec:
 pubsubname: 'pubsub0'
 topic: 'd'
 routes:
  default: '/d'
---
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub3'
spec:
 pubsubname: 'pubsub1'
 topic: c
 routes:
  default: '/c'
---
apiVersion: dapr.io/v1alpha1
kind: Subscription
metadata:
 name: 'sub4'
spec:
 pubsubname: 'pubsub1'
 topic: e
 route: '/e'
`), 0o600))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, g.daprd.GetMetaSubscriptions(t, ctx), 3)
	}, time.Second*5, time.Millisecond*10)
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub0", "d"))
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub1", "c"))
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub1", "e"))

	require.NoError(t, os.WriteFile(filepath.Join(g.resDir2, "sub.yaml"), []byte(`
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub2'
spec:
 pubsubname: 'pubsub0'
 topic: 'd'
 routes:
  default: '/d'
---
apiVersion: dapr.io/v2alpha1
kind: Subscription
metadata:
 name: 'sub3'
spec:
 pubsubname: 'pubsub1'
 topic: c
 routes:
  default: '/c'
---
apiVersion: dapr.io/v1alpha1
kind: Subscription
metadata:
 name: 'sub4'
spec:
 pubsubname: 'pubsub1'
 topic: f
 route: '/f'
`), 0o600))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := g.daprd.GRPCClient(t, ctx).GetMetadata(ctx, new(rtv1.GetMetadataRequest))
		require.NoError(t, err)
		if assert.Len(c, resp.GetSubscriptions(), 3) {
			assert.Equal(c, "f", resp.GetSubscriptions()[2].GetTopic())
		}
	}, time.Second*5, time.Millisecond*10)
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub0", "d"))
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub1", "c"))
	g.sub.ExpectPublishNoReceive(t, ctx, g.daprd, newReq("pubsub1", "e"))
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub1", "f"))

	require.NoError(t, os.Remove(filepath.Join(g.resDir2, "pubsub.yaml")))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, g.daprd.GetMetaRegisteredComponents(t, ctx), 1)
	}, time.Second*5, time.Millisecond*10)
	g.sub.ExpectPublishReceive(t, ctx, g.daprd, newReq("pubsub0", "d"))
	g.sub.ExpectPublishError(t, ctx, g.daprd, newReq("pubsub1", "c"))
	g.sub.ExpectPublishError(t, ctx, g.daprd, newReq("pubsub1", "f"))
}
