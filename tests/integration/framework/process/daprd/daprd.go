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

package daprd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	rtv1 "github.com/nholuongut/dapr/pkg/proto/runtime/v1"
	"github.com/nholuongut/dapr/tests/integration/framework/binary"
	"github.com/nholuongut/dapr/tests/integration/framework/client"
	"github.com/nholuongut/dapr/tests/integration/framework/process"
	"github.com/nholuongut/dapr/tests/integration/framework/process/exec"
	"github.com/nholuongut/dapr/tests/integration/framework/process/ports"
)

type Daprd struct {
	exec       process.Interface
	ports      *ports.Ports
	httpClient *http.Client

	appID            string
	namespace        string
	appProtocol      string
	appPort          *int
	grpcPort         int
	httpPort         int
	internalGRPCPort int
	publicPort       int
	metricsPort      int
	profilePort      int

	cleanupOnce sync.Once
}

func New(t *testing.T, fopts ...Option) *Daprd {
	t.Helper()

	uid, err := uuid.NewRandom()
	require.NoError(t, err)

	fp := ports.Reserve(t, 6)
	opts := options{
		appID:            uid.String(),
		appProtocol:      "http",
		grpcPort:         fp.Port(t),
		httpPort:         fp.Port(t),
		internalGRPCPort: fp.Port(t),
		publicPort:       fp.Port(t),
		metricsPort:      fp.Port(t),
		profilePort:      fp.Port(t),
		logLevel:         "info",
		mode:             "standalone",
	}

	for _, fopt := range fopts {
		fopt(&opts)
	}

	dir := t.TempDir()
	for i, file := range opts.resourceFiles {
		require.NoError(t, os.WriteFile(filepath.Join(dir, strconv.Itoa(i)+".yaml"), []byte(file), 0o600))
	}

	args := []string{
		"--log-level=" + opts.logLevel,
		"--app-id=" + opts.appID,
		"--app-protocol=" + opts.appProtocol,
		"--dapr-grpc-port=" + strconv.Itoa(opts.grpcPort),
		"--dapr-http-port=" + strconv.Itoa(opts.httpPort),
		"--dapr-internal-grpc-port=" + strconv.Itoa(opts.internalGRPCPort),
		"--dapr-internal-grpc-listen-address=127.0.0.1",
		"--dapr-listen-addresses=127.0.0.1",
		"--dapr-public-port=" + strconv.Itoa(opts.publicPort),
		"--dapr-public-listen-address=127.0.0.1",
		"--metrics-port=" + strconv.Itoa(opts.metricsPort),
		"--metrics-listen-address=127.0.0.1",
		"--profile-port=" + strconv.Itoa(opts.profilePort),
		"--enable-app-health-check=" + strconv.FormatBool(opts.appHealthCheck),
		"--app-health-probe-interval=" + strconv.Itoa(opts.appHealthProbeInterval),
		"--app-health-threshold=" + strconv.Itoa(opts.appHealthProbeThreshold),
		"--mode=" + opts.mode,
		"--enable-mtls=" + strconv.FormatBool(opts.enableMTLS),
	}

	if opts.appPort != nil {
		args = append(args, "--app-port="+strconv.Itoa(*opts.appPort))
	}
	if opts.appHealthCheckPath != "" {
		args = append(args, "--app-health-check-path="+opts.appHealthCheckPath)
	}
	if len(opts.resourceFiles) > 0 {
		args = append(args, "--resources-path="+dir)
	}
	for _, dir := range opts.resourceDirs {
		args = append(args, "--resources-path="+dir)
	}
	if len(opts.configs) > 0 {
		for _, c := range opts.configs {
			args = append(args, "--config="+c)
		}
	}
	if len(opts.placementAddresses) > 0 {
		args = append(args, "--placement-host-address="+strings.Join(opts.placementAddresses, ","))
	}
	if len(opts.sentryAddress) > 0 {
		args = append(args, "--sentry-address="+opts.sentryAddress)
	}
	if len(opts.controlPlaneAddress) > 0 {
		args = append(args, "--control-plane-address="+opts.controlPlaneAddress)
	}
	if opts.disableK8sSecretStore != nil {
		args = append(args, "--disable-builtin-k8s-secret-store="+strconv.FormatBool(*opts.disableK8sSecretStore))
	}
	if opts.gracefulShutdownSeconds != nil {
		args = append(args, "--dapr-graceful-shutdown-seconds="+strconv.Itoa(*opts.gracefulShutdownSeconds))
	}
	if opts.blockShutdownDuration != nil {
		args = append(args, "--dapr-block-shutdown-duration="+*opts.blockShutdownDuration)
	}
	if len(opts.schedulerAddresses) > 0 {
		args = append(args, "--scheduler-host-address="+strings.Join(opts.schedulerAddresses, ","))
	}
	if opts.controlPlaneTrustDomain != nil {
		args = append(args, "--control-plane-trust-domain="+*opts.controlPlaneTrustDomain)
	}

	ns := "default"
	if opts.namespace != nil {
		ns = *opts.namespace
		opts.execOpts = append(opts.execOpts, exec.WithEnvVars(t, "NAMESPACE", *opts.namespace))
	}

	return &Daprd{
		exec:             exec.New(t, binary.EnvValue("daprd"), args, opts.execOpts...),
		ports:            fp,
		httpClient:       client.HTTP(t),
		appID:            opts.appID,
		namespace:        ns,
		appProtocol:      opts.appProtocol,
		appPort:          opts.appPort,
		grpcPort:         opts.grpcPort,
		httpPort:         opts.httpPort,
		internalGRPCPort: opts.internalGRPCPort,
		publicPort:       opts.publicPort,
		metricsPort:      opts.metricsPort,
		profilePort:      opts.profilePort,
	}
}

func (d *Daprd) Run(t *testing.T, ctx context.Context) {
	d.ports.Free(t)
	d.exec.Run(t, ctx)
}

func (d *Daprd) Cleanup(t *testing.T) {
	d.cleanupOnce.Do(func() { d.exec.Cleanup(t) })
}

func (d *Daprd) WaitUntilTCPReady(t *testing.T, ctx context.Context) {
	assert.Eventually(t, func() bool {
		dialer := net.Dialer{Timeout: time.Second}
		net, err := dialer.DialContext(ctx, "tcp", d.HTTPAddress())
		if err != nil {
			return false
		}
		net.Close()
		return true
	}, 15*time.Second, 10*time.Millisecond)
}

func (d *Daprd) WaitUntilRunning(t *testing.T, ctx context.Context) {
	client := client.HTTP(t)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/v1.0/healthz", d.HTTPAddress()), nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return http.StatusNoContent == resp.StatusCode
	}, 30*time.Second, 10*time.Millisecond)
}

func (d *Daprd) WaitUntilAppHealth(t *testing.T, ctx context.Context) {
	switch d.appProtocol {
	case "http":
		client := client.HTTP(t)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/v1.0/healthz", d.HTTPAddress()), nil)
		require.NoError(t, err)
		assert.Eventually(t, func() bool {
			resp, err := client.Do(req)
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			return http.StatusNoContent == resp.StatusCode
		}, 10*time.Second, 10*time.Millisecond)

	case "grpc":
		assert.Eventually(t, func() bool {
			//nolint:staticcheck
			conn, err := grpc.Dial(d.AppAddress(t),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock())
			if conn != nil {
				defer conn.Close()
			}

			if err != nil {
				return false
			}
			in := emptypb.Empty{}
			out := rtv1.HealthCheckResponse{}
			err = conn.Invoke(ctx, "/dapr.proto.runtime.v1.AppCallbackHealthCheck/HealthCheck", &in, &out)
			return err == nil
		}, 10*time.Second, 10*time.Millisecond)
	}
}

func (d *Daprd) GRPCConn(t *testing.T, ctx context.Context) *grpc.ClientConn {
	//nolint:staticcheck
	conn, err := grpc.DialContext(ctx, d.GRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	return conn
}

func (d *Daprd) GRPCClient(t *testing.T, ctx context.Context) rtv1.DaprClient {
	return rtv1.NewDaprClient(d.GRPCConn(t, ctx))
}

func (d *Daprd) AppID() string {
	return d.appID
}

func (d *Daprd) Namespace() string {
	return d.namespace
}

func (d *Daprd) ipPort(port int) string {
	return "127.0.0.1:" + strconv.Itoa(port)
}

func (d *Daprd) AppPort(t *testing.T) int {
	t.Helper()
	require.NotNil(t, d.appPort, "no app port specified")
	return *d.appPort
}

func (d *Daprd) AppAddress(t *testing.T) string {
	return d.ipPort(d.AppPort(t))
}

func (d *Daprd) GRPCPort() int {
	return d.grpcPort
}

func (d *Daprd) GRPCAddress() string {
	return d.ipPort(d.GRPCPort())
}

func (d *Daprd) HTTPPort() int {
	return d.httpPort
}

func (d *Daprd) HTTPAddress() string {
	return d.ipPort(d.HTTPPort())
}

func (d *Daprd) InternalGRPCPort() int {
	return d.internalGRPCPort
}

func (d *Daprd) InternalGRPCAddress() string {
	return d.ipPort(d.InternalGRPCPort())
}

func (d *Daprd) PublicPort() int {
	return d.publicPort
}

func (d *Daprd) MetricsPort() int {
	return d.metricsPort
}

func (d *Daprd) MetricsAddress() string {
	return d.ipPort(d.MetricsPort())
}

func (d *Daprd) ProfilePort() int {
	return d.profilePort
}

// Metrics Returns a subset of metrics scraped from the metrics endpoint
func (d *Daprd) Metrics(t *testing.T, ctx context.Context) map[string]float64 {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/metrics", d.MetricsAddress()), nil)
	require.NoError(t, err)

	resp, err := d.httpClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Extract the metrics
	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	metrics := make(map[string]float64)
	for _, mf := range metricFamilies {
		for _, m := range mf.GetMetric() {
			metricName := mf.GetName()
			labels := ""
			for _, l := range m.GetLabel() {
				labels += "|" + l.GetName() + ":" + l.GetValue()
			}
			if counter := m.GetCounter(); counter != nil {
				metrics[metricName+labels] = counter.GetValue()
				continue
			}
			if gauge := m.GetGauge(); gauge != nil {
				metrics[metricName+labels] = gauge.GetValue()
				continue
			}
			h := m.GetHistogram()
			if h == nil {
				continue
			}
			for _, b := range h.GetBucket() {
				bucketKey := metricName + "_bucket" + labels + "|le:" + strconv.FormatUint(uint64(b.GetUpperBound()), 10)
				metrics[bucketKey] = float64(b.GetCumulativeCount())
			}
			metrics[metricName+"_count"+labels] = float64(h.GetSampleCount())
			metrics[metricName+"_sum"+labels] = h.GetSampleSum()
		}
	}

	return metrics
}

func (d *Daprd) HTTPGet2xx(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	d.http2xx(t, ctx, http.MethodGet, path, nil)
}

func (d *Daprd) HTTPPost2xx(t *testing.T, ctx context.Context, path string, body io.Reader, headers ...string) {
	t.Helper()
	d.http2xx(t, ctx, http.MethodPost, path, body, headers...)
}

func (d *Daprd) http2xx(t *testing.T, ctx context.Context, method, path string, body io.Reader, headers ...string) {
	t.Helper()

	require.Zero(t, len(headers)%2, "headers must be key-value pairs")

	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("http://%s/%s", d.HTTPAddress(), path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	require.NoError(t, err)

	for i := 0; i < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}

	resp, err := d.httpClient.Do(req)
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.GreaterOrEqual(t, resp.StatusCode, 200, "expected 2xx status code: "+string(b))
	require.Less(t, resp.StatusCode, 300, "expected 2xx status code: "+string(b))
}

func (d *Daprd) GetMetaRegisteredComponents(t assert.TestingT, ctx context.Context) []*rtv1.RegisteredComponents {
	return d.meta(t, ctx).RegisteredComponents
}

func (d *Daprd) GetMetaSubscriptions(t assert.TestingT, ctx context.Context) []MetadataResponsePubsubSubscription {
	return d.meta(t, ctx).Subscriptions
}

func (d *Daprd) GetMetaSubscriptionsWithType(t assert.TestingT, ctx context.Context, subType string) []MetadataResponsePubsubSubscription {
	subs := d.GetMetaSubscriptions(t, ctx)
	var filteredSubs []MetadataResponsePubsubSubscription
	for _, sub := range subs {
		if sub.Type == subType {
			filteredSubs = append(filteredSubs, sub)
		}
	}
	return filteredSubs
}

func (d *Daprd) GetMetaHTTPEndpoints(t assert.TestingT, ctx context.Context) []*rtv1.MetadataHTTPEndpoint {
	return d.meta(t, ctx).HTTPEndpoints
}

// metaResponse is a subset of metadataResponse defined in pkg/api/http/metadata.go:160
type metaResponse struct {
	RegisteredComponents []*rtv1.RegisteredComponents         `json:"components,omitempty"`
	Subscriptions        []MetadataResponsePubsubSubscription `json:"subscriptions,omitempty"`
	HTTPEndpoints        []*rtv1.MetadataHTTPEndpoint         `json:"httpEndpoints,omitempty"`
}

// MetadataResponsePubsubSubscription copied from pkg/api/http/metadata.go:172 to be able to use in integration tests until we move to Proto format
type MetadataResponsePubsubSubscription struct {
	PubsubName      string                                   `json:"pubsubname"`
	Topic           string                                   `json:"topic"`
	Metadata        map[string]string                        `json:"metadata,omitempty"`
	Rules           []MetadataResponsePubsubSubscriptionRule `json:"rules,omitempty"`
	DeadLetterTopic string                                   `json:"deadLetterTopic"`
	Type            string                                   `json:"type"`
}

type MetadataResponsePubsubSubscriptionRule struct {
	Match string `json:"match,omitempty"`
	Path  string `json:"path,omitempty"`
}

func (d *Daprd) meta(t assert.TestingT, ctx context.Context) metaResponse {
	url := fmt.Sprintf("http://%s/v1.0/metadata", d.HTTPAddress())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	//nolint:testifylint
	if !assert.NoError(t, err) {
		return metaResponse{}
	}

	var meta metaResponse
	resp, err := d.httpClient.Do(req)
	if assert.NoError(t, err) {
		defer resp.Body.Close()
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&meta))
	}

	return meta
}

func (d *Daprd) ActorInvokeURL(actorType, actorID, method string) string {
	return fmt.Sprintf("http://%s/v1.0/actors/%s/%s/method/%s", d.HTTPAddress(), actorType, actorID, method)
}

func (d *Daprd) ActorReminderURL(actorType, actorID, method string) string {
	return fmt.Sprintf("http://%s/v1.0/actors/%s/%s/reminders/%s", d.HTTPAddress(), actorType, actorID, method)
}
