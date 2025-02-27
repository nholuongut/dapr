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

package framework

import (
	"github.com/nholuongut/dapr/tests/integration/framework/process"
	"github.com/nholuongut/dapr/tests/integration/framework/process/once"
)

func WithProcesses(procs ...process.Interface) Option {
	// TODO: if procs string contains `logline` we should move it to the start
	// of the list here otherwise daprd doesn't start up and it just hangs
	// we should make this easier to debug, or just move it to before daprd
	return func(o *options) {
		for _, proc := range procs {
			var found bool
			for _, d := range o.procs {
				if d == proc {
					found = true
					break
				}
			}
			if !found {
				o.procs = append(o.procs, once.Wrap(proc))
			}
		}
	}
}
