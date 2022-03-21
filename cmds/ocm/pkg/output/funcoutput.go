// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package output

import (
	. "github.com/gardener/ocm/cmds/ocm/pkg/data"
)

type FunctionProcessingOutput struct {
	ElementOutput
	function func(interface{})
}

var _ Output = &FunctionProcessingOutput{}

func NewProcessingFunctionOutput(chain ProcessChain, f func(interface{})) *FunctionProcessingOutput {
	return (&FunctionProcessingOutput{}).new(chain, f)
}

func (this *FunctionProcessingOutput) new(chain ProcessChain, f func(interface{})) *FunctionProcessingOutput {
	this.ElementOutput.new(chain)
	this.function = f
	return this
}

func (this *FunctionProcessingOutput) Out(interface{}) error {
	i := this.Elems.Iterator()
	for i.HasNext() {
		this.function(i.Next())
	}
	return nil
}
