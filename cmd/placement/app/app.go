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
	"encoding/json"
	"math"
	"os"

	"github.com/nholuongut/dapr/cmd/placement/options"
	"github.com/nholuongut/dapr/pkg/buildinfo"
	"github.com/nholuongut/dapr/pkg/healthz"
	"github.com/nholuongut/dapr/pkg/healthz/server"
	"github.com/nholuongut/dapr/pkg/metrics"
	"github.com/nholuongut/dapr/pkg/modes"
	"github.com/nholuongut/dapr/pkg/placement"
	"github.com/nholuongut/dapr/pkg/placement/monitoring"
	"github.com/nholuongut/dapr/pkg/placement/raft"
	"github.com/nholuongut/dapr/pkg/security"
	"github.com/dapr/kit/concurrency"
	"github.com/dapr/kit/logger"
	"github.com/dapr/kit/ptr"
	"github.com/dapr/kit/signals"
)

var log = logger.NewLogger("dapr.placement")

func Run() {
	opts, err := options.New(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	// Apply options to all loggers.
	if e := logger.ApplyOptionsToLoggers(&opts.Logger); e != nil {
		log.Fatal(e)
	}

	log.Infof("Starting Dapr Placement Service -- version %s -- commit %s", buildinfo.Version(), buildinfo.Commit())
	log.Infof("Log level set to: %s", opts.Logger.OutputLevel)

	healthz := healthz.New()
	metricsExporter := metrics.New(metrics.Options{
		Log:       log,
		Enabled:   opts.Metrics.Enabled(),
		Namespace: metrics.DefaultMetricNamespace,
		Port:      opts.Metrics.Port(),
		Healthz:   healthz,
	})

	if e := monitoring.InitMetrics(); e != nil {
		log.Fatal(e)
	}

	ctx := signals.Context()
	secProvider, err := security.New(ctx, security.Options{
		SentryAddress:           opts.SentryAddress,
		ControlPlaneTrustDomain: opts.TrustDomain,
		ControlPlaneNamespace:   security.CurrentNamespace(),
		TrustAnchorsFile:        &opts.TrustAnchorsFile,
		AppID:                   "dapr-placement",
		MTLSEnabled:             opts.TLSEnabled,
		Mode:                    modes.DaprMode(opts.Mode),
		Healthz:                 healthz,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Start Raft cluster.
	raftServer := raft.New(raft.Options{
		ID:                opts.RaftID,
		InMem:             opts.RaftInMemEnabled,
		Peers:             opts.RaftPeers,
		LogStorePath:      opts.RaftLogStorePath,
		ReplicationFactor: int64(opts.ReplicationFactor),
		// TODO: fix types
		//nolint:gosec
		MinAPILevel: uint32(opts.MinAPILevel),
		//nolint:gosec
		MaxAPILevel: uint32(opts.MaxAPILevel),
		Healthz:     healthz,
		Security:    secProvider,
	})
	if raftServer == nil {
		log.Fatal("Failed to create raft server.")
	}

	placementOpts := placement.ServiceOpts{
		Port:               opts.PlacementPort,
		RaftNode:           raftServer,
		SecProvider:        secProvider,
		Healthz:            healthz,
		KeepAliveTime:      opts.KeepAliveTime,
		KeepAliveTimeout:   opts.KeepAliveTimeout,
		DisseminateTimeout: opts.DisseminateTimeout,
	}
	if opts.MinAPILevel >= 0 && opts.MinAPILevel < math.MaxInt32 {
		// TODO: fix types
		//nolint:gosec
		placementOpts.MinAPILevel = uint32(opts.MinAPILevel)
	}
	if opts.MaxAPILevel >= 0 && opts.MaxAPILevel < math.MaxInt32 {
		// TODO: fix types
		//nolint:gosec
		placementOpts.MaxAPILevel = ptr.Of(uint32(opts.MaxAPILevel))
	}
	apiServer := placement.NewPlacementService(placementOpts)
	var healthzHandlers []server.Handler
	if opts.MetadataEnabled {
		healthzHandlers = append(healthzHandlers, server.Handler{
			Path: "/placement/state",
			Getter: func() ([]byte, error) {
				var tables *placement.PlacementTables
				tables, err = apiServer.GetPlacementTables()
				if err != nil {
					return nil, err
				}
				return json.Marshal(tables)
			},
		})
	}

	err = concurrency.NewRunnerManager(
		secProvider.Run,
		raftServer.StartRaft,
		metricsExporter.Start,
		server.New(server.Options{
			Log:      log,
			Port:     opts.HealthzPort,
			Healthz:  healthz,
			Handlers: healthzHandlers,
		}).Start,
		apiServer.MonitorLeadership,
		apiServer.Start,
	).Run(ctx)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Placement service shut down gracefully")
}
