//go:build unit
// +build unit

/*
Copyright 2021 The Dapr Authors
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

// Code generated by mockery v1.0.0.

package actors

import (
	"context"
	"errors"
	"sync"
	"time"

	mock "github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/nholuongut/dapr/pkg/actors/internal"
	commonv1pb "github.com/nholuongut/dapr/pkg/proto/common/v1"
	internalsv1pb "github.com/nholuongut/dapr/pkg/proto/internals/v1"
	runtimev1pb "github.com/nholuongut/dapr/pkg/proto/runtime/v1"
	daprt "github.com/nholuongut/dapr/pkg/testing"
)

type (
	// Expose Reminders for mocking
	MockReminder = internal.Reminder

	// Expose PlacementService for mocking
	PlacementService = internal.PlacementService
)

// MockPlacement is a mock placement service.
type MockPlacement struct {
	testAppID            string
	lookupActorResponses map[string]internal.LookupActorResponse
}

func NewMockPlacement(testAppID string) internal.PlacementService {
	return &MockPlacement{
		testAppID:            testAppID,
		lookupActorResponses: make(map[string]internal.LookupActorResponse),
	}
}

// LookupActor implements internal.PlacementService
func (*MockPlacement) AddHostedActorType(string, time.Duration) error {
	return nil
}

func (p *MockPlacement) SetLookupActorResponse(req internal.LookupActorRequest, res internal.LookupActorResponse) {
	p.lookupActorResponses[req.ActorKey()] = res
}

// LookupActor implements internal.PlacementService
func (p *MockPlacement) LookupActor(ctx context.Context, req internal.LookupActorRequest) (internal.LookupActorResponse, error) {
	res, ok := p.lookupActorResponses[req.ActorKey()]
	if ok {
		return res, nil
	}

	return internal.LookupActorResponse{
		Address: "localhost:5000",
		AppID:   p.testAppID,
	}, nil
}

// Start implements internal.PlacementService
func (*MockPlacement) Start(context.Context) error {
	return nil
}

// Stop implements internal.PlacementService
func (*MockPlacement) Close() error {
	return nil
}

// SetOnTableUpdateFn implements internal.PlacementService
func (*MockPlacement) SetOnTableUpdateFn(fn func()) {
	// No-op
}

// SetOnAPILevelUpdate implements internal.PlacementService
func (*MockPlacement) SetOnAPILevelUpdate(fn func(apiLevel uint32)) {
	// No-op
}

// WaitUntilReady implements internal.PlacementService
func (*MockPlacement) WaitUntilReady(ctx context.Context) error {
	return nil
}

// PlacementHealthy implements internal.PlacementService
func (*MockPlacement) PlacementHealthy() bool {
	return true
}

// StatusMessage implements internal.PlacementService
func (*MockPlacement) StatusMessage() string {
	return ""
}

// ReportActorDeactivation implements implements internal.PlacementService
func (*MockPlacement) ReportActorDeactivation(ctx context.Context, actorType, actorID string) error {
	return nil
}

// SetHaltActorFns implements implements internal.PlacementService
func (*MockPlacement) SetHaltActorFns(haltFn internal.HaltActorFn, haltAllFn internal.HaltAllActorsFn) {
	// Nop
}

// MockActors is an autogenerated mock type for the Actors type
type MockActors struct {
	mock.Mock
}

func (_m *MockActors) Entities() []string {
	return nil
}

func (_m *MockActors) RegisterInternalActor(ctx context.Context, actorType string, actor InternalActorFactory, actorIdleTimeout time.Duration) error {
	return nil
}

// Call provides a mock function with given fields: req
func (_m *MockActors) Call(ctx context.Context, req *internalsv1pb.InternalInvokeRequest) (*internalsv1pb.InternalInvokeResponse, error) {
	ret := _m.Called(req)

	var r0 *internalsv1pb.InternalInvokeResponse
	if rf, ok := ret.Get(0).(func(*internalsv1pb.InternalInvokeRequest) *internalsv1pb.InternalInvokeResponse); ok {
		r0 = rf(req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*internalsv1pb.InternalInvokeResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*internalsv1pb.InternalInvokeRequest) error); ok {
		r1 = rf(req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateReminder provides a mock function with given fields: req
func (_m *MockActors) CreateReminder(ctx context.Context, req *CreateReminderRequest) error {
	ret := _m.Called(req)

	var r0 error
	if rf, ok := ret.Get(0).(func(*CreateReminderRequest) error); ok {
		r0 = rf(req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// IsActorHosted provides a mock function with given fields: req
func (_m *MockActors) IsActorHosted(ctx context.Context, req *ActorHostedRequest) bool {
	ret := _m.Called(req)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*ActorHostedRequest) bool); ok {
		r0 = rf(req)
	} else {
		r0 = true
	}

	return r0
}

// CreateTimer provides a mock function with given fields: req
func (_m *MockActors) CreateTimer(ctx context.Context, req *CreateTimerRequest) error {
	ret := _m.Called(req)

	var r0 error
	if rf, ok := ret.Get(0).(func(*CreateTimerRequest) error); ok {
		r0 = rf(req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteReminder provides a mock function with given fields: req
func (_m *MockActors) DeleteReminder(ctx context.Context, req *DeleteReminderRequest) error {
	ret := _m.Called(req)

	var r0 error
	if rf, ok := ret.Get(0).(func(*DeleteReminderRequest) error); ok {
		r0 = rf(req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteTimer provides a mock function with given fields: req
func (_m *MockActors) DeleteTimer(ctx context.Context, req *DeleteTimerRequest) error {
	ret := _m.Called(req)

	var r0 error
	if rf, ok := ret.Get(0).(func(*DeleteTimerRequest) error); ok {
		r0 = rf(req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetState provides a mock function with given fields: req
func (_m *MockActors) GetState(ctx context.Context, req *GetStateRequest) (*StateResponse, error) {
	ret := _m.Called(req)

	var r0 *StateResponse
	if rf, ok := ret.Get(0).(func(*GetStateRequest) *StateResponse); ok {
		r0 = rf(req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*StateResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*GetStateRequest) error); ok {
		r1 = rf(req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// BulkGetState provides a mock function with given fields: req
func (_m *MockActors) GetBulkState(ctx context.Context, req *GetBulkStateRequest) (BulkStateResponse, error) {
	ret := _m.Called(req)

	var r0 BulkStateResponse
	if rf, ok := ret.Get(0).(func(*GetBulkStateRequest) BulkStateResponse); ok {
		r0 = rf(req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(BulkStateResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*GetBulkStateRequest) error); ok {
		r1 = rf(req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteLocalOrRemoteActorReminder provides a mock function with given fields: req
func (_m *MockActors) ExecuteLocalOrRemoteActorReminder(ctx context.Context, reminder *CreateReminderRequest) error {
	ret := _m.Called(ctx, reminder)

	var r0 error
	if rf, ok := ret.Get(1).(func(context.Context, *CreateReminderRequest) error); ok {
		r0 = rf(ctx, reminder)
	} else {
		r0 = ret.Error(1)
	}

	return r0
}

// Init provides a mock function with given fields:
func (_m *MockActors) Init(_ context.Context) error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Close provides a mock function with given fields:
func (_m *MockActors) Close() error {
	_m.Called()
	return nil
}

// TransactionalStateOperation provides a mock function with given fields: req
func (_m *MockActors) TransactionalStateOperation(ctx context.Context, req *TransactionalRequest) error {
	ret := _m.Called(req)

	var r0 error
	if rf, ok := ret.Get(0).(func(*TransactionalRequest) error); ok {
		r0 = rf(req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetReminder provides a mock function with given fields: req
func (_m *MockActors) GetReminder(ctx context.Context, req *GetReminderRequest) (*internal.Reminder, error) {
	ret := _m.Called(req)

	var r0 *internal.Reminder
	if rf, ok := ret.Get(0).(func(*GetReminderRequest) *internal.Reminder); ok {
		r0 = rf(req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*internal.Reminder)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*GetReminderRequest) error); ok {
		r1 = rf(req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRuntimeStatus provides a mock function
func (_m *MockActors) GetRuntimeStatus(ctx context.Context) *runtimev1pb.ActorRuntime {
	_m.Called()
	return &runtimev1pb.ActorRuntime{
		HostReady: true,
		ActiveActors: []*runtimev1pb.ActiveActorsCount{
			{
				Type:  "abcd",
				Count: 10,
			},
			{
				Type:  "xyz",
				Count: 5,
			},
		},
	}
}

type FailingActors struct {
	Failure daprt.Failure
}

func (f *FailingActors) Entities() []string {
	return nil
}

func (f *FailingActors) RegisterInternalActor(ctx context.Context, actorType string, actor InternalActorFactory, actorIdleTimeout time.Duration) error {
	return nil
}

func (f *FailingActors) Call(ctx context.Context, req *internalsv1pb.InternalInvokeRequest) (*internalsv1pb.InternalInvokeResponse, error) {
	if req == nil || req.Actor == nil {
		return nil, errors.New("req.Actor is nil")
	}
	if err := f.Failure.PerformFailure(req.Actor.ActorId); err != nil {
		return nil, err
	}
	var data []byte
	if req.Message != nil && req.Message.Data != nil {
		data = req.Message.Data.Value
	}
	res := &internalsv1pb.InternalInvokeResponse{
		Status: &internalsv1pb.Status{
			Code:    200,
			Message: "Success",
		},
		Message: &commonv1pb.InvokeResponse{
			Data: &anypb.Any{
				Value: data,
			},
		},
	}
	return res, nil
}

func (f *FailingActors) Init(_ context.Context) error {
	return nil
}

func (f *FailingActors) Close() error {
	return nil
}

func (f *FailingActors) GetState(ctx context.Context, req *GetStateRequest) (*StateResponse, error) {
	return nil, nil
}

func (f *FailingActors) GetBulkState(ctx context.Context, req *GetBulkStateRequest) (BulkStateResponse, error) {
	return nil, nil
}

func (f *FailingActors) TransactionalStateOperation(ctx context.Context, req *TransactionalRequest) error {
	return nil
}

func (f *FailingActors) GetReminder(ctx context.Context, req *GetReminderRequest) (*internal.Reminder, error) {
	return nil, nil
}

func (f *FailingActors) CreateReminder(ctx context.Context, req *CreateReminderRequest) error {
	return nil
}

func (f *FailingActors) DeleteReminder(ctx context.Context, req *DeleteReminderRequest) error {
	return nil
}

func (f *FailingActors) CreateTimer(ctx context.Context, req *CreateTimerRequest) error {
	return nil
}

func (f *FailingActors) DeleteTimer(ctx context.Context, req *DeleteTimerRequest) error {
	return nil
}

func (f *FailingActors) IsActorHosted(ctx context.Context, req *ActorHostedRequest) bool {
	return true
}

func (f *FailingActors) ExecuteLocalOrRemoteActorReminder(ctx context.Context, reminder *CreateReminderRequest) error {
	return nil
}

func (f *FailingActors) GetRuntimeStatus(ctx context.Context) *runtimev1pb.ActorRuntime {
	return &runtimev1pb.ActorRuntime{
		ActiveActors: []*runtimev1pb.ActiveActorsCount{},
	}
}

type MockActorFIFOInvocation struct {
	*MockActors
	receivedCallOrderLock  sync.Mutex
	processedCallOrderLock sync.Mutex
	receivedCallOrder      []string
	processedCallOrder     []string
	actorLock              *ActorLock
}

func NewMockActorFIFOInvocation(maxStackDepth int32) *MockActorFIFOInvocation {
	return &MockActorFIFOInvocation{
		MockActors:         &MockActors{},
		receivedCallOrder:  make([]string, 0),
		processedCallOrder: make([]string, 0),
		actorLock:          NewActorLock(maxStackDepth),
	}
}

func (_m *MockActorFIFOInvocation) Call(ctx context.Context, req *internalsv1pb.InternalInvokeRequest) (*internalsv1pb.InternalInvokeResponse, error) {
	ret := _m.Called(req)

	_m.receivedCallOrderLock.Lock()
	_m.receivedCallOrder = append(_m.receivedCallOrder, req.GetMessage().GetMethod())

	// Reuse the method name as a unique requestID so we don't spend cycles on generating UIDs
	uniqueRequestID := req.GetMessage().GetMethod()
	err := _m.actorLock.Lock(&uniqueRequestID)
	_m.receivedCallOrderLock.Unlock()

	if err != nil {
		return nil, err
	}
	defer _m.actorLock.Unlock()

	_m.processedCallOrderLock.Lock()
	_m.processedCallOrder = append(_m.processedCallOrder, req.GetMessage().GetMethod())
	_m.processedCallOrderLock.Unlock()

	// Sleep for 500ns to simulate load and force callers to wait for lock
	time.Sleep(500 * time.Nanosecond)

	var r0 *internalsv1pb.InternalInvokeResponse
	if rf, ok := ret.Get(0).(func(*internalsv1pb.InternalInvokeRequest) *internalsv1pb.InternalInvokeResponse); ok {
		r0 = rf(req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*internalsv1pb.InternalInvokeResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*internalsv1pb.InternalInvokeRequest) error); ok {
		r1 = rf(req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
