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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nholuongut/dapr/tests/integration/framework"
	"github.com/nholuongut/dapr/tests/integration/framework/client"
	"github.com/nholuongut/dapr/tests/integration/framework/file"
	"github.com/nholuongut/dapr/tests/integration/framework/parallel"
	procdaprd "github.com/nholuongut/dapr/tests/integration/framework/process/daprd"
	"github.com/nholuongut/dapr/tests/integration/suite"
)

func init() {
	suite.Register(new(fuzzsecret))
}

type secretValuesFile map[string]string

type fuzzsecret struct {
	daprd *procdaprd.Daprd

	secretStoreName string
	values          secretValuesFile
}

func (f *fuzzsecret) Setup(t *testing.T) []framework.Option {
	const numTests = 1000

	takenNames := make(map[string]bool)
	uid, err := uuid.NewRandom()
	require.NoError(t, err)
	f.secretStoreName = uid.String()

	f.values = make(map[string]string)
	for range numTests {
		var key, value string
		for len(key) == 0 || takenNames[key] {
			fuzz.New().Fuzz(&key)
		}
		fuzz.New().Fuzz(&value)

		// TODO: @joshvanl: resolve string encoding issues for keys and values, so
		// we don't need to base64 encode here.
		key = base64.StdEncoding.EncodeToString([]byte(key))
		value = base64.StdEncoding.EncodeToString([]byte(value))

		f.values[key] = value
		takenNames[key] = true
	}

	secretFileName := file.Paths(t, 1)[0]

	file, err := os.Create(secretFileName)
	require.NoError(t, err)

	je := json.NewEncoder(file)
	je.SetEscapeHTML(false)
	require.NoError(t, je.Encode(f.values))
	require.NoError(t, file.Close())

	f.daprd = procdaprd.New(t, procdaprd.WithResourceFiles((fmt.Sprintf(`
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
  name: '%s'
spec:
  type: secretstores.local.file
  version: v1
  metadata:
  - name: secretsFile
    value: '%s'
`, f.secretStoreName, strings.ReplaceAll(secretFileName, "'", "''"),
	))))
	return []framework.Option{
		framework.WithProcesses(f.daprd),
	}
}

func (f *fuzzsecret) Run(t *testing.T, ctx context.Context) {
	f.daprd.WaitUntilRunning(t, ctx)

	client := client.HTTP(t)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, f.daprd.GetMetaRegisteredComponents(t, ctx), 1)
	}, time.Second*20, time.Millisecond*10)

	pt := parallel.New(t)
	for key, value := range f.values {
		pt.Add(func(t *assert.CollectT) {
			getURL := fmt.Sprintf("http://localhost:%d/v1.0/secrets/%s/%s", f.daprd.HTTPPort(), url.QueryEscape(f.secretStoreName), url.QueryEscape(key))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Equal(t, `{"`+key+`":"`+value+`"}`, strings.TrimSpace(string(respBody)))
		})
	}

	// TODO: Bulk APIs, nesting, multi-valued
}
