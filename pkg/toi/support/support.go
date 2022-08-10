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

package support

import (
	"fmt"
	"io"

	"github.com/mandelsoft/vfs/pkg/vfs"

	"github.com/open-component-model/ocm/pkg/toi/install"

	"github.com/open-component-model/ocm/pkg/common/accessio"
	"github.com/open-component-model/ocm/pkg/contexts/datacontext/vfsattr"

	"github.com/open-component-model/ocm/pkg/common"
	"github.com/open-component-model/ocm/pkg/common/accessobj"
	"github.com/open-component-model/ocm/pkg/contexts/credentials"
	"github.com/open-component-model/ocm/pkg/contexts/credentials/repositories/memory"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ctf"
	ocmutils "github.com/open-component-model/ocm/pkg/contexts/ocm/utils"
	"github.com/open-component-model/ocm/pkg/errors"
	"github.com/open-component-model/ocm/pkg/out"
)

type ExecutorOptions struct {
	Context              ocm.Context
	OutputContext        out.Context
	Action               string
	ComponentVersionName string
	Root                 string
	Inputs               string
	Outputs              string
	OCMConfig            string
	Config               string
	ConfigData           []byte
	Parameters           string
	ParameterData        []byte
	RepoPath             string
	Repository           ocm.Repository
	CredentialRepo       credentials.Repository
	ComponentVersion     ocm.ComponentVersionAccess
	Closer               func() error
}

func (o *ExecutorOptions) FileSystem() vfs.FileSystem {
	return vfsattr.Get(o.Context)
}

func (o *ExecutorOptions) Complete() error {
	if o.ComponentVersionName == "" {
		return fmt.Errorf("component version required")
	}
	compvers, err := common.ParseNameVersion(o.ComponentVersionName)
	if err != nil {
		return err
	}
	if o.OutputContext == nil {
		o.OutputContext = out.New()
	}
	if o.Action == "" {
		o.Action = "install"
	}
	if o.Root == "" {
		o.Root = install.PathTOI
	}
	if o.Inputs == "" {
		o.Inputs = o.Root + "/" + install.Inputs
	}
	if o.Outputs == "" {
		o.Outputs = o.Root + "/" + install.Outputs
	}
	if o.RepoPath == "" {
		o.RepoPath = o.Inputs + "/" + install.InputOCMRepo
	}
	if o.Config == "" {
		cfg := o.Inputs + "/" + install.InputConfig
		if ok, err := vfs.FileExists(o.FileSystem(), cfg); ok && err == nil {
			o.Config = cfg
		}
	}
	if o.Config != "" && o.ConfigData == nil {
		o.ConfigData, err = vfs.ReadFile(o.FileSystem(), o.Config)
		if err != nil {
			return errors.Wrapf(err, "cannot read config %q", o.Config)
		}
	}

	if o.OCMConfig == "" {
		cfg := o.Inputs + "/" + install.InputOCMConfig
		if ok, err := vfs.FileExists(o.FileSystem(), cfg); ok && err == nil {
			o.OCMConfig = cfg
		}
	}
	o.Context, err = ocmutils.Configure(o.Context, o.OCMConfig)
	if err != nil {
		return err
	}

	if o.Parameters == "" {
		p := o.Inputs + "/" + install.InputParameters
		if ok, err := vfs.FileExists(o.FileSystem(), p); ok && err == nil {
			o.Parameters = p
		}
	}
	if o.Parameters != "" && o.ParameterData == nil {
		o.ParameterData, err = vfs.ReadFile(o.FileSystem(), o.Parameters)
		if err != nil {
			return errors.Wrapf(err, "cannot read parameters %q", o.Config)
		}
	}

	var repoCloser io.Closer
	if o.Repository == nil {
		repo, err := ctf.Open(o.Context, accessobj.ACC_READONLY, o.RepoPath, 0, accessio.PathFileSystem(o.FileSystem()))
		if err != nil {
			return errors.Wrapf(err, "cannot open ctf %q", o.RepoPath)
		}
		o.Repository = repo
		repoCloser = repo
	}

	var versCloser io.Closer

	if o.ComponentVersion == nil {
		cv, err := o.Repository.LookupComponentVersion(compvers.GetName(), compvers.GetVersion())
		if err != nil {
			return err
		}
		o.ComponentVersion = cv
		versCloser = cv
	}

	old := o.Closer
	o.Closer = func() error {
		list := errors.ErrListf("closing")
		if versCloser != nil {
			list.Add(errors.Wrapf(versCloser.Close(), "component version"))
		}
		if repoCloser != nil {
			list.Add(errors.Wrapf(repoCloser.Close(), "repository"))
		}
		if old != nil {
			list.Add(errors.Wrapf(old(), "external closer"))
		}
		return list.Result()
	}

	if o.CredentialRepo == nil {
		c, err := o.Context.CredentialsContext().RepositoryForSpec(memory.NewRepositorySpec("default"))
		if err != nil {
			return errors.Wrapf(err, "cannot get default memory based crednetial repository")
		}
		o.CredentialRepo = c
	}
	return nil
}

type Executor struct {
	Completed bool
	Options   *ExecutorOptions
	Run       func(o *ExecutorOptions) error
}

func (e *Executor) Execute() error {
	if e.Options == nil {
		e.Completed = false
		e.Options = &ExecutorOptions{}
	}
	if !e.Completed {
		err := e.Options.Complete()
		if err != nil {
			return err
		}
	}
	list := errors.ErrListf("executor:")
	list.Add(e.Run(e.Options))
	if e.Options.Closer != nil {
		list.Add(e.Options.Closer())
	}
	return list.Result()
}