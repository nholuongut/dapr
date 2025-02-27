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

package v1alpha1

import (
	_ "github.com/nholuongut/dapr/tests/integration/suite/daprd/subscriptions/declarative/selfhosted/v1alpha1/basic"
	_ "github.com/nholuongut/dapr/tests/integration/suite/daprd/subscriptions/declarative/selfhosted/v1alpha1/bulk"
	_ "github.com/nholuongut/dapr/tests/integration/suite/daprd/subscriptions/declarative/selfhosted/v1alpha1/deadletter"
	_ "github.com/nholuongut/dapr/tests/integration/suite/daprd/subscriptions/declarative/selfhosted/v1alpha1/missing"
	_ "github.com/nholuongut/dapr/tests/integration/suite/daprd/subscriptions/declarative/selfhosted/v1alpha1/rawpayload"
	_ "github.com/nholuongut/dapr/tests/integration/suite/daprd/subscriptions/declarative/selfhosted/v1alpha1/route"
	_ "github.com/nholuongut/dapr/tests/integration/suite/daprd/subscriptions/declarative/selfhosted/v1alpha1/scopes"
)
