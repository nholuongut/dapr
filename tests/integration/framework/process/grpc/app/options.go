/*
Copyright 2023 The Dapr Authors
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

package app

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	testpb "github.com/nholuongut/dapr/tests/integration/framework/process/grpc/app/proto"

	commonv1 "github.com/nholuongut/dapr/pkg/proto/common/v1"
	rtv1 "github.com/nholuongut/dapr/pkg/proto/runtime/v1"
	procgrpc "github.com/nholuongut/dapr/tests/integration/framework/process/grpc"
)

// options contains the options for running a GRPC server app in integration
// tests.
type options struct {
	grpcopts           []procgrpc.Option
	withRegister       func(s *grpc.Server)
	onTopicEventFn     func(context.Context, *rtv1.TopicEventRequest) (*rtv1.TopicEventResponse, error)
	onBulkTopicEventFn func(context.Context, *rtv1.TopicEventBulkRequest) (*rtv1.TopicEventBulkResponse, error)
	onInvokeFn         func(context.Context, *commonv1.InvokeRequest) (*commonv1.InvokeResponse, error)
	onJobEventFn       func(context.Context, *rtv1.JobEventRequest) (*rtv1.JobEventResponse, error)
	listTopicSubFn     func(ctx context.Context, in *emptypb.Empty) (*rtv1.ListTopicSubscriptionsResponse, error)
	listInputBindFn    func(context.Context, *emptypb.Empty) (*rtv1.ListInputBindingsResponse, error)
	onBindingEventFn   func(context.Context, *rtv1.BindingEventRequest) (*rtv1.BindingEventResponse, error)
	healthCheckFn      func(context.Context, *emptypb.Empty) (*rtv1.HealthCheckResponse, error)
	pingFn             func(context.Context, *testpb.PingRequest) (*testpb.PingResponse, error)
}

func WithGRPCOptions(opts ...procgrpc.Option) func(*options) {
	return func(o *options) {
		o.grpcopts = opts
	}
}

func WithOnTopicEventFn(fn func(context.Context, *rtv1.TopicEventRequest) (*rtv1.TopicEventResponse, error)) func(*options) {
	return func(opts *options) {
		opts.onTopicEventFn = fn
	}
}

func WithOnBulkTopicEventFn(fn func(context.Context, *rtv1.TopicEventBulkRequest) (*rtv1.TopicEventBulkResponse, error)) func(*options) {
	return func(opts *options) {
		opts.onBulkTopicEventFn = fn
	}
}

func WithOnInvokeFn(fn func(ctx context.Context, in *commonv1.InvokeRequest) (*commonv1.InvokeResponse, error)) func(*options) {
	return func(opts *options) {
		opts.onInvokeFn = fn
	}
}

func WithOnJobEventFn(fn func(ctx context.Context, in *rtv1.JobEventRequest) (*rtv1.JobEventResponse, error)) func(*options) {
	return func(opts *options) {
		opts.onJobEventFn = fn
	}
}

func WithListTopicSubscriptions(fn func(ctx context.Context, in *emptypb.Empty) (*rtv1.ListTopicSubscriptionsResponse, error)) func(*options) {
	return func(opts *options) {
		opts.listTopicSubFn = fn
	}
}

func WithListInputBindings(fn func(context.Context, *emptypb.Empty) (*rtv1.ListInputBindingsResponse, error)) func(*options) {
	return func(opts *options) {
		opts.listInputBindFn = fn
	}
}

func WithOnBindingEventFn(fn func(context.Context, *rtv1.BindingEventRequest) (*rtv1.BindingEventResponse, error)) func(*options) {
	return func(opts *options) {
		opts.onBindingEventFn = fn
	}
}

func WithHealthCheckFn(fn func(context.Context, *emptypb.Empty) (*rtv1.HealthCheckResponse, error)) func(*options) {
	return func(opts *options) {
		opts.healthCheckFn = fn
	}
}

func WithPingFn(fn func(context.Context, *testpb.PingRequest) (*testpb.PingResponse, error)) func(*options) {
	return func(opts *options) {
		opts.pingFn = fn
	}
}

func WithRegister(fn func(s *grpc.Server)) func(*options) {
	return func(opts *options) {
		opts.withRegister = fn
	}
}
