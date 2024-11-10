//go:build perf
// +build perf

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

package service_invocation_grpc_perf

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nholuongut/dapr/tests/perf"
	"github.com/nholuongut/dapr/tests/perf/utils"
	kube "github.com/nholuongut/dapr/tests/platforms/kubernetes"
	"github.com/nholuongut/dapr/tests/runner"
	"github.com/nholuongut/dapr/tests/runner/summary"
)

const numHealthChecks = 60 // Number of times to check for endpoint health per app.

const testLabel = "service-invocation-grpc"

var tr *runner.TestRunner

func TestMain(m *testing.M) {
	utils.SetupLogs("service_invocation_grpc")

	testApps := []kube.AppDescription{
		{
			AppName:           "testapp",
			DaprEnabled:       true,
			ImageName:         "perf-service_invocation_grpc",
			Replicas:          1,
			IngressEnabled:    true,
			MetricsEnabled:    true,
			AppPort:           3000,
			AppProtocol:       "grpc",
			DaprCPULimit:      "4.0",
			DaprCPURequest:    "0.1",
			DaprMemoryLimit:   "512Mi",
			DaprMemoryRequest: "250Mi",
			AppCPULimit:       "4.0",
			AppCPURequest:     "0.1",
			AppMemoryLimit:    "800Mi",
			AppMemoryRequest:  "2500Mi",
			Labels: map[string]string{
				"daprtest": testLabel + "-testapp",
			},
		},
		{
			AppName:           "tester",
			DaprEnabled:       true,
			ImageName:         "perf-tester",
			Replicas:          1,
			IngressEnabled:    true,
			MetricsEnabled:    true,
			AppPort:           3001,
			DaprCPULimit:      "4.0",
			DaprCPURequest:    "0.1",
			DaprMemoryLimit:   "512Mi",
			DaprMemoryRequest: "250Mi",
			AppCPULimit:       "4.0",
			AppCPURequest:     "0.1",
			AppMemoryLimit:    "800Mi",
			AppMemoryRequest:  "2500Mi",
			Labels: map[string]string{
				"daprtest": testLabel + "-tester",
			},
			PodAffinityLabels: map[string]string{
				"daprtest": testLabel + "-testapp",
			},
		},
	}

	tr = runner.NewTestRunner("serviceinvocationgrpc", testApps, nil, nil)
	os.Exit(tr.Start(m))
}

func TestServiceInvocationGrpcPerformance(t *testing.T) {
	p := perf.Params(
		perf.WithQPS(1000),
		perf.WithConnections(16),
		perf.WithDuration("1m"),
		perf.WithPayloadSize(1024),
	)
	t.Logf("running service invocation grpc test with params: qps=%v, connections=%v, duration=%s, payload size=%v, payload=%v", p.QPS, p.ClientConnections, p.TestDuration, p.PayloadSizeKB, p.Payload)

	// Get the ingress external url of test app
	testAppURL := tr.Platform.AcquireAppExternalURL("testapp")
	require.NotEmpty(t, testAppURL, "test app external URL must not be empty")

	// Check if test app endpoint is available
	t.Logf("Waiting until test app grpc service is available: %s", testAppURL)
	_, err := utils.GrpcAccessNTimes(testAppURL, utils.GrpcServiceInvoke, numHealthChecks)
	require.NoError(t, err)

	// Get the ingress external url of tester app
	testerAppURL := tr.Platform.AcquireAppExternalURL("tester")
	require.NotEmpty(t, testerAppURL, "tester app external URL must not be empty")

	// Check if tester app endpoint is available
	t.Logf("teter app url: %s", testerAppURL)
	_, err = utils.HTTPGetNTimes(testerAppURL, numHealthChecks)
	require.NoError(t, err)

	// Perform baseline test
	p.Grpc = true
	p.Dapr = "capability=invoke,target=appcallback,method=load"
	p.TargetEndpoint = "http://testapp:3000"
	body, err := json.Marshal(&p)
	require.NoError(t, err)

	t.Log("running baseline test...")
	baselineResp, err := utils.HTTPPost(fmt.Sprintf("%s/test", testerAppURL), body)
	t.Logf("baseline test results: %s", string(baselineResp))
	t.Log("checking err...")
	require.NoError(t, err)
	require.NotEmpty(t, baselineResp)
	// fast fail if daprResp starts with error
	require.False(t, strings.HasPrefix(string(baselineResp), "error"))

	// Perform an initial run with Dapr as warmup
	warmup := perf.Params(
		perf.WithQPS(100),
		perf.WithConnections(16),
		perf.WithDuration("10s"),
		perf.WithPayloadSize(1024),
	)
	warmup.Grpc = true
	warmup.Dapr = "capability=invoke,target=dapr,method=load,appid=testapp"
	warmup.TargetEndpoint = "http://localhost:50001"
	body, err = json.Marshal(warmup)
	require.NoError(t, err)

	t.Log("running warmup...")
	warmupResp, err := utils.HTTPPost(fmt.Sprintf("%s/test", testerAppURL), body)
	t.Logf("warmup results: %s", string(warmupResp))
	require.NoError(t, err)
	require.NotEmpty(t, warmupResp)
	// fast fail if warmupResp starts with error
	require.False(t, strings.HasPrefix(string(warmupResp), "error"))

	// Perform dapr test
	p.Grpc = true
	p.Dapr = "capability=invoke,target=dapr,method=load,appid=testapp"
	p.TargetEndpoint = "http://localhost:50001"
	body, err = json.Marshal(p)
	require.NoError(t, err)

	t.Log("running dapr test...")
	daprResp, err := utils.HTTPPost(fmt.Sprintf("%s/test", testerAppURL), body)
	t.Logf("dapr test results: %s", string(daprResp))
	t.Log("checking err...")
	require.NoError(t, err)
	require.NotEmpty(t, daprResp)

	sidecarUsage, err := tr.Platform.GetSidecarUsage("testapp")
	require.NoError(t, err)

	appUsage, err := tr.Platform.GetAppUsage("testapp")
	require.NoError(t, err)

	restarts, err := tr.Platform.GetTotalRestarts("testapp")
	require.NoError(t, err)

	t.Logf("target dapr sidecar consumed %vm Cpu and %vMb of Memory", sidecarUsage.CPUm, sidecarUsage.MemoryMb)

	var daprResult perf.TestResult
	err = json.Unmarshal(daprResp, &daprResult)
	require.NoError(t, err)

	var baselineResult perf.TestResult
	err = json.Unmarshal(baselineResp, &baselineResult)
	require.NoError(t, err)

	percentiles := []string{"50th", "75th", "90th", "99th"}
	var tp90Latency float64

	for k, v := range percentiles {
		daprValue := daprResult.DurationHistogram.Percentiles[k].Value
		baselineValue := baselineResult.DurationHistogram.Percentiles[k].Value

		latency := (daprValue - baselineValue) * 1000
		if v == "90th" {
			tp90Latency = latency
		}
		t.Logf("added latency for %s percentile: %sms", v, fmt.Sprintf("%.2f", latency))
	}
	avg := (daprResult.DurationHistogram.Avg - baselineResult.DurationHistogram.Avg) * 1000
	baselineLatency := baselineResult.DurationHistogram.Avg * 1000
	daprLatency := daprResult.DurationHistogram.Avg * 1000
	t.Logf("baseline latency avg: %sms", fmt.Sprintf("%.2f", baselineLatency))
	t.Logf("dapr latency avg: %sms", fmt.Sprintf("%.2f", daprLatency))
	t.Logf("added latency avg: %sms", fmt.Sprintf("%.2f", avg))

	summary.ForTest(t).
		Service("testapp").
		CPU(appUsage.CPUm).
		Memory(appUsage.MemoryMb).
		SidecarCPU(sidecarUsage.CPUm).
		SidecarMemory(sidecarUsage.MemoryMb).
		Restarts(restarts).
		BaselineLatency(baselineLatency).
		DaprLatency(daprLatency).
		AddedLatency(avg).
		ActualQPS(daprResult.ActualQPS).
		Params(p).
		OutputFortio(daprResult).
		Flush()

	require.Equal(t, 0, daprResult.RetCodes.Num400)
	require.Equal(t, 0, daprResult.RetCodes.Num500)
	require.Equal(t, 0, restarts)
	require.True(t, daprResult.ActualQPS > float64(p.QPS)*0.99)
	require.Greater(t, tp90Latency, 0.0)
	require.LessOrEqual(t, tp90Latency, 2.5)
}
