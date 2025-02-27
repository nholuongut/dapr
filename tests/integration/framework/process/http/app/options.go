/*
Copyright 2024 The Dapr Authors
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
	nethttp "net/http"

	"github.com/nholuongut/dapr/tests/integration/framework/process/http"
)

type options struct {
	subscribe     string
	config        string
	initialHealth bool
	handlerFuncs  []http.Option
}

func WithSubscribe(subscribe string) Option {
	return func(o *options) {
		o.subscribe = subscribe
	}
}

func WithConfig(config string) Option {
	return func(o *options) {
		o.config = config
	}
}

func WithHandlerFunc(path string, fn nethttp.HandlerFunc) Option {
	return func(o *options) {
		o.handlerFuncs = append(o.handlerFuncs, http.WithHandlerFunc(path, fn))
	}
}

func WithInitialHealth(initialHealth bool) Option {
	return func(o *options) {
		o.initialHealth = initialHealth
	}
}
