/*
Copyright 2022 The Dapr Authors
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

package universal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dapr/components-contrib/state"
	"github.com/nholuongut/dapr/pkg/api/errors"
	stateLoader "github.com/nholuongut/dapr/pkg/components/state"
	diag "github.com/nholuongut/dapr/pkg/diagnostics"
	"github.com/nholuongut/dapr/pkg/encryption"
	runtimev1pb "github.com/nholuongut/dapr/pkg/proto/runtime/v1"
	"github.com/nholuongut/dapr/pkg/resiliency"
	kiterrors "github.com/dapr/kit/errors"
)

func (a *Universal) GetStateStore(name string) (state.Store, error) {
	if a.compStore.StateStoresLen() == 0 {
		err := errors.StateStore(name).NotConfigured("")
		a.logger.Debug(err)
		return nil, err
	}

	stateStore, ok := a.compStore.GetStateStore(name)
	if !ok {
		err := errors.StateStore(name).NotFound("")
		a.logger.Debug(err)
		return nil, err
	}

	return stateStore, nil
}

func (a *Universal) QueryStateAlpha1(ctx context.Context, in *runtimev1pb.QueryStateRequest) (*runtimev1pb.QueryStateResponse, error) {
	store, err := a.GetStateStore(in.GetStoreName())
	if err != nil {
		// Error has already been logged
		return nil, err
	}

	querier, ok := store.(state.Querier)
	if !ok {
		err = errors.StateStore(in.GetStoreName()).QueryUnsupported()
		a.logger.Debug(err)
		return nil, err
	}

	if encryption.EncryptedStateStore(in.GetStoreName()) {
		err = errors.StateStore(in.GetStoreName()).QueryFailed("cannot query encrypted store")
		a.logger.Debug(err)
		return nil, err
	}

	var req state.QueryRequest
	if err = json.Unmarshal([]byte(in.GetQuery()), &req.Query); err != nil {
		err = errors.StateStore(in.GetStoreName()).QueryFailed("failed to parse JSON query body: " + err.Error())
		a.logger.Debug(err)
		return nil, err
	}

	req.Metadata = in.GetMetadata()

	start := time.Now()
	policyRunner := resiliency.NewRunner[*state.QueryResponse](ctx,
		a.resiliency.ComponentOutboundPolicy(in.GetStoreName(), resiliency.Statestore),
	)
	resp, err := policyRunner(func(ctx context.Context) (*state.QueryResponse, error) {
		return querier.Query(ctx, &req)
	})
	elapsed := diag.ElapsedSince(start)

	diag.DefaultComponentMonitoring.StateInvoked(ctx, in.GetStoreName(), diag.StateQuery, err == nil, elapsed)

	if err != nil {
		if kerr, ok := kiterrors.FromError(err); ok {
			return nil, kerr
		}

		err = errors.StateStore(in.GetStoreName()).QueryFailed(err.Error())
		a.logger.Debug(err)
		return nil, err
	}

	if resp == nil || len(resp.Results) == 0 {
		return &runtimev1pb.QueryStateResponse{}, nil
	}

	ret := &runtimev1pb.QueryStateResponse{
		Results:  make([]*runtimev1pb.QueryStateItem, len(resp.Results)),
		Token:    resp.Token,
		Metadata: resp.Metadata,
	}

	for i := range resp.Results {
		row := &runtimev1pb.QueryStateItem{
			Key:   stateLoader.GetOriginalStateKey(resp.Results[i].Key),
			Data:  resp.Results[i].Data,
			Error: resp.Results[i].Error,
		}
		if resp.Results[i].ETag != nil {
			row.Etag = *resp.Results[i].ETag
		}
		ret.Results[i] = row
	}

	return ret, nil
}
