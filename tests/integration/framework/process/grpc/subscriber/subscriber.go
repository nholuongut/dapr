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

package subscriber

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	rtv1 "github.com/nholuongut/dapr/pkg/proto/runtime/v1"
	"github.com/nholuongut/dapr/tests/integration/framework/process/daprd"
	"github.com/nholuongut/dapr/tests/integration/framework/process/grpc/app"
)

type Option func(*options)

type Subscriber struct {
	app      *app.App
	healthy  *atomic.Bool
	inCh     chan *rtv1.TopicEventRequest
	inBulkCh chan *rtv1.TopicEventBulkRequest
	closeCh  chan struct{}
	closed   atomic.Bool
}

func New(t *testing.T, fopts ...Option) *Subscriber {
	t.Helper()

	opts := options{
		initialHealth: true,
	}
	for _, fopt := range fopts {
		fopt(&opts)
	}

	inCh := make(chan *rtv1.TopicEventRequest, 100)
	inBulkCh := make(chan *rtv1.TopicEventBulkRequest, 100)
	closeCh := make(chan struct{})

	var healthy atomic.Bool
	healthy.Store(opts.initialHealth)

	return &Subscriber{
		inCh:     inCh,
		inBulkCh: inBulkCh,
		closeCh:  closeCh,
		healthy:  &healthy,
		app: app.New(t,
			app.WithHealthCheckFn(func(context.Context, *emptypb.Empty) (*rtv1.HealthCheckResponse, error) {
				if healthy.Load() {
					return &rtv1.HealthCheckResponse{}, nil
				}
				return nil, assert.AnError
			}),
			app.WithListTopicSubscriptions(opts.listTopicSubFn),
			app.WithOnTopicEventFn(func(ctx context.Context, in *rtv1.TopicEventRequest) (*rtv1.TopicEventResponse, error) {
				select {
				case inCh <- in:
				case <-ctx.Done():
				case <-closeCh:
				}
				return new(rtv1.TopicEventResponse), nil
			}),
			app.WithOnBulkTopicEventFn(func(ctx context.Context, in *rtv1.TopicEventBulkRequest) (*rtv1.TopicEventBulkResponse, error) {
				select {
				case inBulkCh <- in:
				case <-ctx.Done():
				case <-closeCh:
				}
				stats := make([]*rtv1.TopicEventBulkResponseEntry, len(in.GetEntries()))
				for i, e := range in.GetEntries() {
					stats[i] = &rtv1.TopicEventBulkResponseEntry{
						EntryId: e.GetEntryId(),
						Status:  rtv1.TopicEventResponse_SUCCESS,
					}
				}
				return &rtv1.TopicEventBulkResponse{Statuses: stats}, nil
			}),
		),
	}
}

func (s *Subscriber) Run(t *testing.T, ctx context.Context) {
	t.Helper()
	s.app.Run(t, ctx)
}

func (s *Subscriber) Cleanup(t *testing.T) {
	t.Helper()
	if s.closed.CompareAndSwap(false, true) {
		close(s.closeCh)
		s.app.Cleanup(t)
	}
}

func (s *Subscriber) Port(t *testing.T) int {
	t.Helper()
	return s.app.Port(t)
}

func (s *Subscriber) Receive(t *testing.T, ctx context.Context) *rtv1.TopicEventRequest {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		require.Fail(t, "timed out waiting for event response")
		return nil
	case in := <-s.inCh:
		return in
	}
}

func (s *Subscriber) ReceiveBulk(t *testing.T, ctx context.Context) *rtv1.TopicEventBulkRequest {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	select {
	case <-ctx.Done():
		require.Fail(t, "timed out waiting for event response")
		return nil
	case in := <-s.inBulkCh:
		return in
	}
}

func (s *Subscriber) AssertEventChanLen(t *testing.T, l int) {
	t.Helper()
	assert.Len(t, s.inCh, l)
}

func (s *Subscriber) AssertBulkEventChanLen(t *testing.T, l int) {
	t.Helper()
	assert.Len(t, s.inBulkCh, l)
}

func (s *Subscriber) ExpectPublishReceive(t *testing.T, ctx context.Context, daprd *daprd.Daprd, req *rtv1.PublishEventRequest) {
	t.Helper()
	_, err := daprd.GRPCClient(t, ctx).PublishEvent(ctx, req)
	require.NoError(t, err)
	s.Receive(t, ctx)
	s.AssertEventChanLen(t, 0)
}

func (s *Subscriber) ExpectPublishError(t *testing.T, ctx context.Context, daprd *daprd.Daprd, req *rtv1.PublishEventRequest) {
	t.Helper()
	_, err := daprd.GRPCClient(t, ctx).PublishEvent(ctx, req)
	require.Error(t, err)
}

func (s *Subscriber) ExpectPublishNoReceive(t *testing.T, ctx context.Context, daprd *daprd.Daprd, req *rtv1.PublishEventRequest) {
	t.Helper()
	_, err := daprd.GRPCClient(t, ctx).PublishEvent(ctx, req)
	require.NoError(t, err)
	s.AssertEventChanLen(t, 0)
}

func (s *Subscriber) SetHealth(healthy bool) {
	s.healthy.Store(healthy)
}
