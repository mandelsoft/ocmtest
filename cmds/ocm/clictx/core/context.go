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

package core

import (
	"context"
	"reflect"
	"sync"

	"github.com/gardener/ocm/pkg/common/accessio"
	"github.com/gardener/ocm/pkg/common/accessobj"
	"github.com/gardener/ocm/pkg/config"
	cfgcpi "github.com/gardener/ocm/pkg/config/cpi"
	"github.com/gardener/ocm/pkg/credentials"
	"github.com/gardener/ocm/pkg/datacontext"
	"github.com/gardener/ocm/pkg/errors"
	"github.com/gardener/ocm/pkg/oci"
	ctfoci "github.com/gardener/ocm/pkg/oci/repositories/ctf"
	"github.com/gardener/ocm/pkg/oci/repositories/docker"
	ociregoci "github.com/gardener/ocm/pkg/oci/repositories/ocireg"
	"github.com/gardener/ocm/pkg/ocm"
	ctfocm "github.com/gardener/ocm/pkg/ocm/repositories/ctf"
	"github.com/gardener/ocm/pkg/ocm/repositories/ocireg"
	"github.com/gardener/ocm/pkg/runtime"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"sigs.k8s.io/yaml"
)

const CONTEXT_TYPE = "ocm.cmd.context.gardener.cloud"

type OCI interface {
	Context() oci.Context
	AddRepository(name string, spec oci.RepositorySpec) error
	GetRepository(name string) (oci.Repository, error)
	DetermineRepository(typ string, spec string) (oci.Repository, error)
	OpenCTF(path string) (oci.Repository, error)
}

type OCM interface {
	Context() ocm.Context
	AddRepository(name string, spec ocm.RepositorySpec) error
	GetRepository(name string) (ocm.Repository, error)
	DetermineRepository(typ string, spec string) (ocm.Repository, error)
	OpenCTF(path string) (ocm.Repository, error)
}

type Context interface {
	datacontext.Context

	AttributesContext() datacontext.AttributesContext

	ConfigContext() config.Context
	CredentialsContext() credentials.Context
	OCIContext() oci.Context
	OCMContext() ocm.Context

	FileSystem() vfs.FileSystem

	OCI() OCI
	OCM() OCM

	ApplyOption(options *accessio.Options)
}

var key = reflect.TypeOf(_context{})

// DefaultContext is the default context initialized by init functions
var DefaultContext = Builder{}.New()

// ForContext returns the Context to use for context.Context.
// This is eiter an explicit context or the default context.
// The returned context incorporates the given context.
func ForContext(ctx context.Context) Context {
	return datacontext.ForContextByKey(ctx, key, DefaultContext).(Context)
}

////////////////////////////////////////////////////////////////////////////////

type _context struct {
	lock sync.RWMutex
	datacontext.Context
	updater cfgcpi.Updater

	sharedAttributes datacontext.AttributesContext

	config      config.Context
	credentials credentials.Context
	oci         *_oci
	ocm         *_ocm

	filesystem vfs.FileSystem

	ocirepos map[string]oci.RepositorySpec
	ocmrepos map[string]ocm.RepositorySpec
}

var _ Context = &_context{}

func newContext(shared datacontext.AttributesContext, ocmctx ocm.Context, fs vfs.FileSystem) Context {
	if fs == nil {
		fs = osfs.New()
	}
	c := &_context{
		sharedAttributes: shared,
		credentials:      ocmctx.CredentialsContext(),
		config:           ocmctx.CredentialsContext().ConfigContext(),
		updater:          cfgcpi.NewUpdate(ocmctx.CredentialsContext().ConfigContext()),

		filesystem: fs,
		ocirepos:   map[string]oci.RepositorySpec{},
		ocmrepos:   map[string]ocm.RepositorySpec{},
	}
	c.Context = datacontext.NewContextBase(c, CONTEXT_TYPE, key, shared.GetAttributes())
	c.oci = newOCI(c, ocmctx)
	c.ocm = newOCM(c, ocmctx)
	return c
}

func (c *_context) AttributesContext() datacontext.AttributesContext {
	return c.sharedAttributes
}

func (c *_context) ConfigContext() config.Context {
	return c.config
}

func (c *_context) CredentialsContext() credentials.Context {
	return c.credentials
}

func (c *_context) OCIContext() oci.Context {
	return c.oci.Context()
}

func (c *_context) OCMContext() ocm.Context {
	return c.ocm.Context()
}

func (c *_context) FileSystem() vfs.FileSystem {
	return c.filesystem
}

func (c *_context) OCI() OCI {
	return c.oci
}

func (c *_context) OCM() OCM {
	return c.ocm
}

func (c *_context) ApplyOption(options *accessio.Options) {
	options.PathFileSystem = c.FileSystem()
}

////////////////////////////////////////////////////////////////////////////////
// the coding for _oci and _ocm is identical except the package used:
// _oci uses oci and ctfoci
// _ocm uses ocm and ctfocm

type _oci struct {
	*_context
	ctx   oci.Context
	repos map[string]oci.RepositorySpec
}

func newOCI(ctx *_context, ocmctx ocm.Context) *_oci {
	return &_oci{
		_context: ctx,
		ctx:      ocmctx.OCIContext(),
		repos:    map[string]oci.RepositorySpec{},
	}
}

func (c *_oci) Context() oci.Context {
	return c.ctx
}

func (c *_oci) AddRepository(name string, spec oci.RepositorySpec) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.repos[name] = spec
	return nil
}

func (c *_oci) GetRepository(name string) (oci.Repository, error) {
	err := c.updater.Update(c)
	if err != nil {
		return nil, err
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	spec := c.ocirepos[name]

	if spec == nil {
		return nil, errors.ErrUnknown("oci repository", name)
	}
	return c.ctx.RepositoryForSpec(spec)
}

func (c *_oci) DetermineRepository(typ string, spec string) (oci.Repository, error) {
	var rspec oci.RepositorySpec
	var parsed interface{}
	var repobase oci.Repository

	if ctfoci.GetFormat(accessio.FileFormat(typ)) != nil {
		rspec = ctfoci.NewRepositorySpec(accessobj.ACC_WRITABLE|accessobj.ACC_CREATE, spec, accessio.FileFormat(typ), accessio.PathFileSystem(c.FileSystem()))
	} else {
		switch typ {
		case "Docker", "DockerDeamon":
			rspec = docker.NewRepositorySpec(spec)
		case "OCIRegistry":
			rspec = ociregoci.NewRepositorySpec(spec)
		case "":
			err := yaml.Unmarshal([]byte(spec), &parsed)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot unmarshal repository spec")
			}
			if s, ok := parsed.(string); ok {
				repobase, err = c.GetRepository(s)
				if err == nil {
					return repobase, err
				}
				return c.OpenCTF(spec)
			} else {
				rspec, err = c.ctx.RepositoryTypes().DecodeRepositorySpec([]byte(spec), runtime.DefaultJSONEncoding)
				if err != nil {
					return nil, err
				}
			}
		default:
			return nil, errors.ErrNotSupported("repository type", typ)
		}
	}
	return c.ctx.RepositoryForSpec(rspec)
}

func (c *_oci) OpenCTF(path string) (oci.Repository, error) {
	ok, err := vfs.Exists(c.FileSystem(), path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.ErrNotFound("file", path)
	}
	return ctfoci.Open(c.ctx, accessobj.ACC_WRITABLE, path, 0, accessio.PathFileSystem(c.FileSystem()))
}

////////////////////////////////////////////////////////////////////////////////

type _ocm struct {
	*_context
	ctx   ocm.Context
	repos map[string]ocm.RepositorySpec
}

func newOCM(ctx *_context, ocmctx ocm.Context) *_ocm {
	return &_ocm{
		_context: ctx,
		ctx:      ocmctx,
		repos:    map[string]ocm.RepositorySpec{},
	}
}
func (c *_ocm) Context() ocm.Context {
	return c.ctx
}

func (c *_ocm) AddRepository(name string, spec ocm.RepositorySpec) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.repos[name] = spec
	return nil
}

func (c *_ocm) GetRepository(name string) (ocm.Repository, error) {
	err := c.updater.Update(c)
	if err != nil {
		return nil, err
	}
	c.lock.RLock()
	defer c.lock.RUnlock()

	spec := c.ocmrepos[name]

	if spec == nil {
		return nil, errors.ErrUnknown("ocm repository", name)
	}
	return c.ctx.RepositoryForSpec(spec)
}

func (c *_ocm) DetermineRepository(typ string, spec string) (ocm.Repository, error) {
	var rspec ocm.RepositorySpec
	var parsed interface{}
	var repobase ocm.Repository

	if ctfoci.GetFormat(accessio.FileFormat(typ)) != nil {
		rspec = ctfocm.NewRepositorySpec(accessobj.ACC_WRITABLE|accessobj.ACC_CREATE, spec, accessio.FileFormat(typ), c)
	} else {
		switch typ {
		case "OCIRegistry":
			rspec = ocireg.NewRepositorySpec(spec, nil)
		case "":
			err := yaml.Unmarshal([]byte(spec), &parsed)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot unmarshal repository spec")
			}
			if s, ok := parsed.(string); ok {
				repobase, err = c.GetRepository(s)
				if err == nil {
					return repobase, err
				}
				return c.OpenCTF(spec)
			} else {
				rspec, err = c.ctx.RepositoryTypes().DecodeRepositorySpec([]byte(spec), runtime.DefaultJSONEncoding)
				if err != nil {
					return nil, err
				}
			}
		default:
			return nil, errors.ErrNotSupported("repository type", typ)
		}
	}
	return c.ctx.RepositoryForSpec(rspec)
}

func (c *_ocm) OpenCTF(path string) (ocm.Repository, error) {
	ok, err := vfs.Exists(c.FileSystem(), path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.ErrNotFound("file", path)
	}
	return ctfocm.Open(c.ctx, accessobj.ACC_WRITABLE, path, 0, c)
}
