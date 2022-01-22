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

package cpi

// This is the Context Provider Interface for credential providers

import (
	"github.com/gardener/ocm/pkg/common"
	"github.com/gardener/ocm/pkg/credentials/core"
)

const DirectCredentialsType = core.DirectCredentialsType

const KIND_CREDENTIALS = core.KIND_CREDENTIALS

type Context = core.Context
type Repository = core.Repository
type RepositoryType = core.RepositoryType
type Credentials = core.Credentials
type CredentialsSpec = core.CredentialsSpec
type RepositorySpec = core.RepositorySpec
type GenericRepositorySpec = core.GenericRepositorySpec
type GenericCredentialsSpec = core.GenericCredentialsSpec

var DefaultContext = core.DefaultContext

func NewGenericCredentialsSpec(name string, repospec GenericRepositorySpec) *GenericCredentialsSpec {
	return core.NewGenericCredentialsSpec(name, repospec)
}

func NewCredentialsSpec(name string, repospec RepositorySpec) CredentialsSpec {
	return core.NewCredentialsSpec(name, repospec)
}

func RegisterRepositoryType(name string, atype RepositoryType) {
	core.DefaultRepositoryTypeScheme.Register(name, atype)
}

func NewCredentials(props common.Properties) Credentials {
	return core.NewCredentials(props)
}

func ErrUnknownCredentials(name string) error {
	return core.ErrUnknownCredentials(name)
}
