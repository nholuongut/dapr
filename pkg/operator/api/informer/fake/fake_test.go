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

package fake

import (
	"testing"

	compapi "github.com/nholuongut/dapr/pkg/apis/components/v1alpha1"
	"github.com/nholuongut/dapr/pkg/operator/api/informer"
)

func Test_Fake(t *testing.T) {
	var _ informer.Interface[compapi.Component] = New[compapi.Component]()
}
