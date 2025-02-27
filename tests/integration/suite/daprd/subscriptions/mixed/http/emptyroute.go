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

package http

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nholuongut/dapr/tests/integration/framework"
	"github.com/nholuongut/dapr/tests/integration/framework/process/daprd"
	"github.com/nholuongut/dapr/tests/integration/framework/process/http/subscriber"
	"github.com/nholuongut/dapr/tests/integration/suite"
)

func init() {
	suite.Register(new(emptyroute))
}

type emptyroute struct {
	daprd *daprd.Daprd
	sub   *subscriber.Subscriber
}

func (e *emptyroute) Setup(t *testing.T) []framework.Option {
	e.sub = subscriber.New(t,
		subscriber.WithRoutes("/a/b/c/d", "/123"),
		subscriber.WithProgrammaticSubscriptions(
			subscriber.SubscriptionJSON{
				PubsubName: "mypub",
				Topic:      "a",
				Route:      "/a/b/c/d",
			},
			subscriber.SubscriptionJSON{
				PubsubName: "mypub",
				Topic:      "b",
				Routes: subscriber.RoutesJSON{
					Rules: []*subscriber.RuleJSON{
						{Path: "/123"},
					},
				},
			},
		),
	)
	e.daprd = daprd.New(t,
		daprd.WithAppPort(e.sub.Port()),
		daprd.WithResourceFiles(`
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: mypub
spec:
 type: pubsub.in-memory
 version: v1
---
apiVersion: dapr.io/v2alpha1
Kind: Subscription
metadata:
 name: sub1
spec:
 pubsubname: mypub
 topic: a
 routes:
  rules:
  - path: /123
---
apiVersion: dapr.io/v1alpha1
Kind: Subscription
metadata:
 name: sub2
spec:
 pubsubname: mypub
 topic: b
 route: /a/b/c/d
`))

	return []framework.Option{
		framework.WithProcesses(e.sub, e.daprd),
	}
}

func (e *emptyroute) Run(t *testing.T, ctx context.Context) {
	e.daprd.WaitUntilRunning(t, ctx)

	e.sub.Publish(t, ctx, subscriber.PublishRequest{
		Daprd:      e.daprd,
		PubSubName: "mypub",
		Topic:      "a",
	})
	assert.Equal(t, "/a/b/c/d", e.sub.Receive(t, ctx).Route)

	e.sub.Publish(t, ctx, subscriber.PublishRequest{
		Daprd:      e.daprd,
		PubSubName: "mypub",
		Topic:      "b",
	})
	assert.Equal(t, "/123", e.sub.Receive(t, ctx).Route)
}
