//go:build allcomponents

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

package components

import (
	"context"

	contribmiddleware "github.com/dapr/components-contrib/middleware"
	"github.com/dapr/components-contrib/middleware/http/opa"
	httpMiddlewareLoader "github.com/nholuongut/dapr/pkg/components/middleware/http"
	"github.com/nholuongut/dapr/pkg/middleware"
	"github.com/dapr/kit/logger"
)

func init() {
	httpMiddlewareLoader.DefaultRegistry.RegisterComponent(func(log logger.Logger) httpMiddlewareLoader.FactoryMethod {
		return func(metadata contribmiddleware.Metadata) (middleware.HTTP, error) {
			return opa.NewMiddleware(log).GetHandler(context.TODO(), metadata)
		}
	}, "opa")
}
