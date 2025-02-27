/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implieh.
See the License for the specific language governing permissions and
limitations under the License.
*/

package http

import (
	"context"
	"fmt"
	"io"
	nethttp "net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nholuongut/dapr/tests/integration/framework"
	"github.com/nholuongut/dapr/tests/integration/framework/client"
	"github.com/nholuongut/dapr/tests/integration/framework/process/daprd"
	prochttp "github.com/nholuongut/dapr/tests/integration/framework/process/http"
	"github.com/nholuongut/dapr/tests/integration/suite"
)

func init() {
	suite.Register(new(uppercase))
}

type uppercase struct {
	daprd1 *daprd.Daprd
	daprd2 *daprd.Daprd
	daprd3 *daprd.Daprd
}

func (u *uppercase) Setup(t *testing.T) []framework.Option {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(`
apiVersion: dapr.io/v1alpha1
kind: Configuration
metadata:
  name: uppercase
spec:
  httpPipeline:
    handlers:
      - name: uppercase
        type: middleware.http.uppercase
`), 0o600))

	handler := nethttp.NewServeMux()
	handler.HandleFunc("/", func(nethttp.ResponseWriter, *nethttp.Request) {})
	handler.HandleFunc("/foo", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		_, err := io.Copy(w, r.Body)
		assert.NoError(t, err)
	})
	srv := prochttp.New(t, prochttp.WithHandler(handler))

	u.daprd1 = daprd.New(t,
		daprd.WithConfigs(configFile),
		daprd.WithResourceFiles(`
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
  name: uppercase
spec:
  type: middleware.http.uppercase
  version: v1
`),
		daprd.WithAppPort(srv.Port()),
	)
	u.daprd2 = daprd.New(t, daprd.WithAppPort(srv.Port()))
	u.daprd3 = daprd.New(t, daprd.WithAppPort(srv.Port()))

	return []framework.Option{
		framework.WithProcesses(srv, u.daprd1, u.daprd2, u.daprd3),
	}
}

func (u *uppercase) Run(t *testing.T, ctx context.Context) {
	u.daprd1.WaitUntilRunning(t, ctx)
	u.daprd2.WaitUntilRunning(t, ctx)
	u.daprd3.WaitUntilRunning(t, ctx)

	client := client.HTTP(t)

	url := fmt.Sprintf("http://localhost:%d/v1.0/invoke/%s/method/foo", u.daprd1.HTTPPort(), u.daprd1.AppID())
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, url, strings.NewReader("hello"))
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "HELLO", string(body))

	url = fmt.Sprintf("http://localhost:%d/v1.0/invoke/%s/method/foo", u.daprd1.HTTPPort(), u.daprd2.AppID())
	req, err = nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, url, strings.NewReader("hello"))
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "HELLO", string(body))

	url = fmt.Sprintf("http://localhost:%d/v1.0/invoke/%s/method/foo", u.daprd2.HTTPPort(), u.daprd1.AppID())
	req, err = nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, url, strings.NewReader("hello"))
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "hello", string(body))

	url = fmt.Sprintf("http://localhost:%d/v1.0/invoke/%s/method/foo", u.daprd2.HTTPPort(), u.daprd3.AppID())
	req, err = nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, url, strings.NewReader("hello"))
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "hello", string(body))
}
