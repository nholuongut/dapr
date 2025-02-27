/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://wwb.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package singular

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nholuongut/dapr/tests/integration/framework"
	"github.com/nholuongut/dapr/tests/integration/framework/process/daprd"
	"github.com/nholuongut/dapr/tests/integration/framework/process/exec"
	"github.com/nholuongut/dapr/tests/integration/framework/process/logline"
	"github.com/nholuongut/dapr/tests/integration/suite"
)

func init() {
	suite.Register(new(actors))
}

// actors ensures that 2 actor workflow backends cannot be loaded at the same time.
type actors struct {
	logline *logline.LogLine
	daprd   *daprd.Daprd
}

func (a *actors) Setup(t *testing.T) []framework.Option {
	a.logline = logline.New(t,
		logline.WithStdoutLineContains(
			"Fatal error from runtime: process component wfbackend2 error: [INIT_COMPONENT_FAILURE]: initialization error occurred for wfbackend2 (workflowbackend.actors/v1): cannot create more than one workflow backend component",
		),
	)

	a.daprd = daprd.New(t,
		daprd.WithResourceFiles(`
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: wfbackend1
spec:
 type: workflowbackend.actors
 version: v1
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
 name: wfbackend2
spec:
 type: workflowbackend.actors
 version: v1
`),
		daprd.WithExecOptions(
			exec.WithExitCode(1),
			exec.WithRunError(func(t *testing.T, err error) {
				require.ErrorContains(t, err, "exit status 1")
			}),
			exec.WithStdout(a.logline.Stdout()),
		),
	)

	return []framework.Option{
		framework.WithProcesses(a.logline, a.daprd),
	}
}

func (a *actors) Run(t *testing.T, ctx context.Context) {
	a.logline.EventuallyFoundAll(t)
}
