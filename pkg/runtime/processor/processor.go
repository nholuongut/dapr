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

package processor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	grpcmanager "github.com/nholuongut/dapr/pkg/api/grpc/manager"
	componentsapi "github.com/nholuongut/dapr/pkg/apis/components/v1alpha1"
	httpendpointsapi "github.com/nholuongut/dapr/pkg/apis/httpEndpoint/v1alpha1"
	"github.com/nholuongut/dapr/pkg/components"
	"github.com/nholuongut/dapr/pkg/config"
	"github.com/nholuongut/dapr/pkg/middleware/http"
	"github.com/nholuongut/dapr/pkg/modes"
	"github.com/nholuongut/dapr/pkg/outbox"
	operatorv1 "github.com/nholuongut/dapr/pkg/proto/operator/v1"
	"github.com/nholuongut/dapr/pkg/resiliency"
	"github.com/nholuongut/dapr/pkg/runtime/channels"
	"github.com/nholuongut/dapr/pkg/runtime/compstore"
	"github.com/nholuongut/dapr/pkg/runtime/meta"
	"github.com/nholuongut/dapr/pkg/runtime/processor/binding"
	"github.com/nholuongut/dapr/pkg/runtime/processor/configuration"
	"github.com/nholuongut/dapr/pkg/runtime/processor/conversation"
	"github.com/nholuongut/dapr/pkg/runtime/processor/crypto"
	"github.com/nholuongut/dapr/pkg/runtime/processor/lock"
	"github.com/nholuongut/dapr/pkg/runtime/processor/middleware"
	"github.com/nholuongut/dapr/pkg/runtime/processor/pubsub"
	"github.com/nholuongut/dapr/pkg/runtime/processor/secret"
	"github.com/nholuongut/dapr/pkg/runtime/processor/state"
	"github.com/nholuongut/dapr/pkg/runtime/processor/subscriber"
	"github.com/nholuongut/dapr/pkg/runtime/processor/wfbackend"
	rtpubsub "github.com/nholuongut/dapr/pkg/runtime/pubsub"
	"github.com/nholuongut/dapr/pkg/runtime/registry"
	"github.com/nholuongut/dapr/pkg/security"
	"github.com/dapr/kit/concurrency"
	"github.com/dapr/kit/logger"
)

const (
	defaultComponentInitTimeout = time.Second * 5
)

var log = logger.NewLogger("dapr.runtime.processor")

type Options struct {
	// ID is the ID of this Dapr instance.
	ID string

	// Namespace is the namespace of this Dapr instance.
	Namespace string

	// Mode is the mode of this Dapr instance.
	Mode modes.DaprMode

	// PodName is the name of the pod.
	PodName string

	// ActorsEnabled indicates whether placement service is enabled in this Dapr cluster.
	ActorsEnabled bool

	// IsHTTP indicates whether the connection to the application is using the
	// HTTP protocol.
	IsHTTP bool

	// Registry is the all-component registry.
	Registry *registry.Registry

	// ComponentStore is the component store.
	ComponentStore *compstore.ComponentStore

	// Metadata is the metadata helper.
	Meta *meta.Meta

	// GlobalConfig is the global configuration.
	GlobalConfig *config.Configuration

	Resiliency resiliency.Provider

	GRPC *grpcmanager.Manager

	Channels *channels.Channels

	OperatorClient operatorv1.OperatorClient

	MiddlewareHTTP *http.HTTP

	Security security.Handler

	Outbox outbox.Outbox

	Adapter         rtpubsub.Adapter
	AdapterStreamer rtpubsub.AdapterStreamer

	// Reporter is the reporter for the operator.
	Reporter registry.Reporter
}

// Processor manages the lifecycle of all components categories.
type Processor struct {
	appID           string
	compStore       *compstore.ComponentStore
	managers        map[components.Category]manager
	state           StateManager
	secret          SecretManager
	binding         BindingManager
	workflowBackend WorkflowBackendManager
	security        security.Handler
	subscriber      *subscriber.Subscriber
	reporter        registry.Reporter

	pendingHTTPEndpoints       chan httpendpointsapi.HTTPEndpoint
	pendingComponents          chan componentsapi.Component
	pendingComponentsWaiting   sync.WaitGroup
	pendingComponentDependents map[string][]componentsapi.Component
	subErrCh                   chan error

	lock     sync.RWMutex
	chlock   sync.RWMutex
	running  atomic.Bool
	shutdown atomic.Bool
	closedCh chan struct{}
}

func New(opts Options) *Processor {
	subscriber := subscriber.New(subscriber.Options{
		AppID:           opts.ID,
		Namespace:       opts.Namespace,
		Resiliency:      opts.Resiliency,
		TracingSpec:     opts.GlobalConfig.Spec.TracingSpec,
		IsHTTP:          opts.IsHTTP,
		Channels:        opts.Channels,
		GRPC:            opts.GRPC,
		CompStore:       opts.ComponentStore,
		Adapter:         opts.Adapter,
		AdapterStreamer: opts.AdapterStreamer,
	})

	state := state.New(state.Options{
		ActorsEnabled:  opts.ActorsEnabled,
		Registry:       opts.Registry.StateStores(),
		ComponentStore: opts.ComponentStore,
		Meta:           opts.Meta,
		Outbox:         opts.Outbox,
	})

	secret := secret.New(secret.Options{
		Registry:       opts.Registry.SecretStores(),
		ComponentStore: opts.ComponentStore,
		Meta:           opts.Meta,
		OperatorClient: opts.OperatorClient,
	})

	binding := binding.New(binding.Options{
		Registry:       opts.Registry.Bindings(),
		ComponentStore: opts.ComponentStore,
		Meta:           opts.Meta,
		IsHTTP:         opts.IsHTTP,
		Resiliency:     opts.Resiliency,
		GRPC:           opts.GRPC,
		TracingSpec:    opts.GlobalConfig.Spec.TracingSpec,
		Channels:       opts.Channels,
	})

	wfbe := wfbackend.New(wfbackend.Options{
		AppID:          opts.ID,
		Registry:       opts.Registry.WorkflowBackends(),
		ComponentStore: opts.ComponentStore,
		Meta:           opts.Meta,
	})

	// ensure a default no-op reporter
	reporter := DefaultReporter
	if opts.Reporter != nil {
		reporter = opts.Reporter
	}

	return &Processor{
		appID:                      opts.ID,
		pendingHTTPEndpoints:       make(chan httpendpointsapi.HTTPEndpoint),
		pendingComponents:          make(chan componentsapi.Component),
		pendingComponentDependents: make(map[string][]componentsapi.Component),
		subErrCh:                   make(chan error),
		closedCh:                   make(chan struct{}),
		compStore:                  opts.ComponentStore,
		state:                      state,
		binding:                    binding,
		secret:                     secret,
		workflowBackend:            wfbe,
		security:                   opts.Security,
		subscriber:                 subscriber,
		reporter:                   reporter,
		managers: map[components.Category]manager{
			components.CategoryBindings: binding,
			components.CategoryConfiguration: configuration.New(configuration.Options{
				Registry:       opts.Registry.Configurations(),
				ComponentStore: opts.ComponentStore,
				Meta:           opts.Meta,
			}),
			components.CategoryCryptoProvider: crypto.New(crypto.Options{
				Registry:       opts.Registry.Crypto(),
				ComponentStore: opts.ComponentStore,
				Meta:           opts.Meta,
			}),
			components.CategoryLock: lock.New(lock.Options{
				Registry:       opts.Registry.Locks(),
				ComponentStore: opts.ComponentStore,
				Meta:           opts.Meta,
			}),
			components.CategoryPubSub: pubsub.New(pubsub.Options{
				AppID:          opts.ID,
				Registry:       opts.Registry.PubSubs(),
				Meta:           opts.Meta,
				ComponentStore: opts.ComponentStore,
				Subscriber:     subscriber,
			}),
			components.CategorySecretStore:     secret,
			components.CategoryStateStore:      state,
			components.CategoryWorkflowBackend: wfbe,
			components.CategoryMiddleware: middleware.New(middleware.Options{
				Meta:         opts.Meta,
				RegistryHTTP: opts.Registry.HTTPMiddlewares(),
				HTTP:         opts.MiddlewareHTTP,
			}),
			components.CategoryConversation: conversation.New(conversation.Options{
				Meta:     opts.Meta,
				Registry: opts.Registry.Conversations(),
				Store:    opts.ComponentStore,
			}),
		},
	}
}

func (p *Processor) Process(ctx context.Context) error {
	if !p.running.CompareAndSwap(false, true) {
		return errors.New("processor is already running")
	}

	return concurrency.NewRunnerManager(
		p.processComponents,
		p.processHTTPEndpoints,
		p.processSubscriptions,
		p.subscriber.Run,
		func(ctx context.Context) error {
			<-ctx.Done()
			close(p.closedCh)
			p.chlock.Lock()
			defer p.chlock.Unlock()
			p.shutdown.Store(true)
			close(p.pendingComponents)
			close(p.pendingHTTPEndpoints)
			return nil
		},
	).Run(ctx)
}

// DefaultReporter is the default resource reporter for the registry. It does nothing.
func DefaultReporter(context.Context, componentsapi.Component, *operatorv1.ResourceResult) error {
	return nil
}
