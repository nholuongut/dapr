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
	"github.com/nholuongut/dapr/cmd/operator/options"
	"github.com/nholuongut/dapr/pkg/buildinfo"
	"github.com/nholuongut/dapr/pkg/healthz"
	"github.com/nholuongut/dapr/pkg/healthz/server"
	"github.com/nholuongut/dapr/pkg/metrics"
	"github.com/nholuongut/dapr/pkg/operator"
	"github.com/nholuongut/dapr/pkg/operator/monitoring"
	"github.com/dapr/kit/concurrency"
	"github.com/dapr/kit/logger"
	"github.com/dapr/kit/signals"
)

var log = logger.NewLogger("dapr.operator")

func Run() {
	opts := options.New()

	// Apply options to all loggers.
	if err := logger.ApplyOptionsToLoggers(&opts.Logger); err != nil {
		log.Fatal(err)
	}

	log.Infof("Starting Dapr Operator -- version %s -- commit %s", buildinfo.Version(), buildinfo.Commit())
	log.Infof("Log level set to: %s", opts.Logger.OutputLevel)

	healthz := healthz.New()
	metricsExporter := metrics.New(metrics.Options{
		Log:       log,
		Enabled:   opts.Metrics.Enabled(),
		Namespace: metrics.DefaultMetricNamespace,
		Port:      opts.Metrics.Port(),
		Healthz:   healthz,
	})

	if err := monitoring.InitMetrics(); err != nil {
		log.Fatal(err)
	}

	ctx := signals.Context()
	op, err := operator.NewOperator(ctx, operator.Options{
		Config:                              opts.Config,
		TrustAnchorsFile:                    opts.TrustAnchorsFile,
		LeaderElection:                      !opts.DisableLeaderElection,
		WatchdogMaxRestartsPerMin:           opts.MaxPodRestartsPerMinute,
		WatchNamespace:                      opts.WatchNamespace,
		ServiceReconcilerEnabled:            !opts.DisableServiceReconciler,
		ArgoRolloutServiceReconcilerEnabled: opts.EnableArgoRolloutServiceReconciler,
		WatchdogEnabled:                     opts.WatchdogEnabled,
		WatchdogInterval:                    opts.WatchdogInterval,
		WatchdogCanPatchPodLabels:           opts.WatchdogCanPatchPodLabels,
		APIPort:                             opts.APIPort,
		APIListenAddress:                    opts.APIListenAddress,
		WebhookServerPort:                   opts.WebhookServerPort,
		WebhookServerListenAddress:          opts.WebhookServerListenAddress,
		Healthz:                             healthz,
	})
	if err != nil {
		log.Fatalf("error creating operator: %v", err)
	}

	err = concurrency.NewRunnerManager(
		metricsExporter.Start,
		op.Start,
		server.New(server.Options{
			Log:     log,
			Port:    opts.HealthzPort,
			Healthz: healthz,
		}).Start,
	).Run(ctx)
	if err != nil {
		log.Fatalf("error running operator: %v", err)
	}

	log.Info("operator shut down gracefully")
}
