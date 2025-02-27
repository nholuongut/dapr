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

package bindings_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	b "github.com/dapr/components-contrib/bindings"
	"github.com/nholuongut/dapr/pkg/components/bindings"
	"github.com/dapr/kit/logger"
)

type (
	mockInputBinding struct {
		b.InputBinding
	}

	mockOutputBinding struct {
		b.OutputBinding
	}
)

func TestRegistry(t *testing.T) {
	testRegistry := bindings.NewRegistry()

	t.Run("input binding is registered", func(t *testing.T) {
		const (
			inputBindingName   = "mockInputBinding"
			inputBindingNameV2 = "mockInputBinding/v2"
			componentName      = "bindings." + inputBindingName
		)

		// Initiate mock object
		mockInput := &mockInputBinding{}
		mockInputV2 := &mockInputBinding{}

		// act
		testRegistry.RegisterInputBinding(func(_ logger.Logger) b.InputBinding {
			return mockInput
		}, inputBindingName)
		testRegistry.RegisterInputBinding(func(_ logger.Logger) b.InputBinding {
			return mockInputV2
		}, inputBindingNameV2)

		// assert v0 and v1
		assert.True(t, testRegistry.HasInputBinding(componentName, "v0"))
		p, e := testRegistry.CreateInputBinding(componentName, "v0", "")
		require.NoError(t, e)
		assert.Same(t, mockInput, p)
		p, e = testRegistry.CreateInputBinding(componentName, "v1", "")
		require.NoError(t, e)
		assert.Same(t, mockInput, p)

		// assert v2
		assert.True(t, testRegistry.HasInputBinding(componentName, "v2"))
		pV2, e := testRegistry.CreateInputBinding(componentName, "v2", "")
		require.NoError(t, e)
		assert.Same(t, mockInputV2, pV2)

		// check case-insensitivity
		pV2, e = testRegistry.CreateInputBinding(strings.ToUpper(componentName), "V2", "")
		require.NoError(t, e)
		assert.Same(t, mockInputV2, pV2)
	})

	t.Run("input binding is not registered", func(t *testing.T) {
		const (
			inputBindingName = "fakeInputBinding"
			componentName    = "bindings." + inputBindingName
		)

		// act
		assert.False(t, testRegistry.HasInputBinding(componentName, "v0"))
		assert.False(t, testRegistry.HasInputBinding(componentName, "v1"))
		assert.False(t, testRegistry.HasInputBinding(componentName, "v2"))
		p, actualError := testRegistry.CreateInputBinding(componentName, "v1", "")
		expectedError := fmt.Errorf("couldn't find input binding %s/v1", componentName)

		// assert
		assert.Nil(t, p)
		assert.Equal(t, expectedError.Error(), actualError.Error())
	})

	t.Run("output binding is registered", func(t *testing.T) {
		const (
			outputBindingName   = "mockInputBinding"
			outputBindingNameV2 = "mockInputBinding/v2"
			componentName       = "bindings." + outputBindingName
		)

		// Initiate mock object
		mockOutput := &mockOutputBinding{}
		mockOutputV2 := &mockOutputBinding{}

		// act
		testRegistry.RegisterOutputBinding(func(_ logger.Logger) b.OutputBinding {
			return mockOutput
		}, outputBindingName)
		testRegistry.RegisterOutputBinding(func(_ logger.Logger) b.OutputBinding {
			return mockOutputV2
		}, outputBindingNameV2)

		// assert v0 and v1
		assert.True(t, testRegistry.HasOutputBinding(componentName, "v0"))
		p, e := testRegistry.CreateOutputBinding(componentName, "v0", "")
		require.NoError(t, e)
		assert.Same(t, mockOutput, p)
		assert.True(t, testRegistry.HasOutputBinding(componentName, "v1"))
		p, e = testRegistry.CreateOutputBinding(componentName, "v1", "")
		require.NoError(t, e)
		assert.Same(t, mockOutput, p)

		// assert v2
		assert.True(t, testRegistry.HasOutputBinding(componentName, "v2"))
		pV2, e := testRegistry.CreateOutputBinding(componentName, "v2", "")
		require.NoError(t, e)
		assert.Same(t, mockOutputV2, pV2)
	})

	t.Run("output binding is not registered", func(t *testing.T) {
		const (
			outputBindingName = "fakeOutputBinding"
			componentName     = "bindings." + outputBindingName
		)

		// act
		assert.False(t, testRegistry.HasOutputBinding(componentName, "v0"))
		assert.False(t, testRegistry.HasOutputBinding(componentName, "v1"))
		assert.False(t, testRegistry.HasOutputBinding(componentName, "v2"))
		p, actualError := testRegistry.CreateOutputBinding(componentName, "v1", "")
		expectedError := fmt.Errorf("couldn't find output binding %s/v1", componentName)

		// assert
		assert.Nil(t, p)
		assert.Equal(t, expectedError.Error(), actualError.Error())
	})
}
