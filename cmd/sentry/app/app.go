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
	"os"
	"path/filepath"
	"time"

	"github.com/nholuongut/dapr/cmd/sentry/options"
	"github.com/nholuongut/dapr/pkg/buildinfo"
	"github.com/nholuongut/dapr/pkg/healthz"
	healthzserver "github.com/nholuongut/dapr/pkg/healthz/server"
	"github.com/nholuongut/dapr/pkg/metrics"
	"github.com/nholuongut/dapr/pkg/sentry"
	"github.com/nholuongut/dapr/pkg/sentry/config"
	"github.com/nholuongut/dapr/pkg/sentry/monitoring"
	"github.com/nholuongut/dapr/utils"
	"github.com/dapr/kit/concurrency"
	"github.com/dapr/kit/fswatcher"
	"github.com/dapr/kit/logger"
	"github.com/dapr/kit/signals"
)

var log = logger.NewLogger("dapr.sentry")

func Run() {
	opts := options.New(os.Args[1:])

	// Apply options to all loggers
	if err := logger.ApplyOptionsToLoggers(&opts.Logger); err != nil {
		log.Fatal(err)
	}

	log.Infof("Starting Dapr Sentry certificate authority -- version %s -- commit %s", buildinfo.Version(), buildinfo.Commit())
	log.Infof("Log level set to: %s", opts.Logger.OutputLevel)

	if err := utils.SetEnvVariables(map[string]string{
		utils.KubeConfigVar: opts.Kubeconfig,
	}); err != nil {
		log.Fatalf("Error setting env: %v", err)
	}

	if err := monitoring.InitMetrics(); err != nil {
		log.Fatal(err)
	}

	issuerCertPath := filepath.Join(opts.IssuerCredentialsPath, opts.IssuerCertFilename)
	issuerKeyPath := filepath.Join(opts.IssuerCredentialsPath, opts.IssuerKeyFilename)
	rootCertPath := filepath.Join(opts.IssuerCredentialsPath, opts.RootCAFilename)

	cfg, err := config.FromConfigName(opts.ConfigName)
	if err != nil {
		log.Fatal(err)
	}

	cfg.IssuerCertPath = issuerCertPath
	cfg.IssuerKeyPath = issuerKeyPath
	cfg.RootCertPath = rootCertPath
	cfg.TrustDomain = opts.TrustDomain
	cfg.Port = opts.Port
	cfg.ListenAddress = opts.ListenAddress

	var (
		watchDir    = filepath.Dir(cfg.IssuerCertPath)
		issuerEvent = make(chan struct{})
		mngr        = concurrency.NewRunnerManager()
	)

	// We use runner manager inception here since we want the inner manager to be
	// restarted when the CA server needs to be restarted because of file events.
	// We don't want to restart the healthz server and file watcher on file
	// events (as well as wanting to terminate the program on signals).
	caMngrFactory := func(ctx context.Context) error {
		healthz := healthz.New()
		metricsExporter := metrics.New(metrics.Options{
			Log:       log,
			Enabled:   opts.Metrics.Enabled(),
			Namespace: metrics.DefaultMetricNamespace,
			Port:      opts.Metrics.Port(),
			Healthz:   healthz,
		})
		sentry, serr := sentry.New(ctx, sentry.Options{
			Config:  cfg,
			Healthz: healthz,
		})
		if serr != nil {
			return serr
		}
		return concurrency.NewRunnerManager(
			healthzserver.New(healthzserver.Options{
				Log:     log,
				Port:    opts.HealthzPort,
				Healthz: healthz,
			}).Start,
			metricsExporter.Start,
			sentry.Start,
			func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return nil

				case <-issuerEvent:
					monitoring.IssuerCertChanged()
					log.Debug("Received issuer credentials changed signal")

					select {
					case <-ctx.Done():
						return nil
					// Batch all signals within 2s of each other
					case <-time.After(2 * time.Second):
						log.Warn("Issuer credentials changed; reloading")
						return nil
					}
				}
			},
		).Run(ctx)
	}

	// CA Server
	err = mngr.Add(func(ctx context.Context) error {
		for {
			if runErr := caMngrFactory(ctx); runErr != nil {
				return runErr
			}
			// Catch outer context cancellation to exit.
			select {
			case <-ctx.Done():
				return nil
			default:
			}
		}
	})
	if err != nil {
		log.Fatal(err)
	}

	// Watch for changes in the watchDir
	fs, err := fswatcher.New(fswatcher.Options{
		Targets: []string{watchDir},
	})
	if err != nil {
		log.Fatal(err)
	}
	if err = mngr.Add(func(ctx context.Context) error {
		log.Infof("Starting watch on filesystem directory: %s", watchDir)
		return fs.Run(ctx, issuerEvent)
	}); err != nil {
		log.Fatal(err)
	}

	// Run the runner manager.
	if err := mngr.Run(signals.Context()); err != nil {
		log.Fatal(err)
	}
	log.Info("Sentry shut down gracefully")
}
