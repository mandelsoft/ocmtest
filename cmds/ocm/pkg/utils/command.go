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

package utils

import (
	"os"
	"strings"

	"github.com/open-component-model/ocm/cmds/ocm/clictx"
	"github.com/open-component-model/ocm/cmds/ocm/pkg/options"
	"github.com/open-component-model/ocm/pkg/out"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// OCMCommand is a command pattern, thta can be instantiated for a dediated
// sub command name.
type OCMCommand interface {
	clictx.Context
	ForName(name string) *cobra.Command
	AddFlags(fs *pflag.FlagSet)
	Complete(args []string) error
	Run() error
}

type BaseCommand struct {
	clictx.Context
	options.OptionSet
}

func NewBaseCommand(ctx clictx.Context, opts ...options.Options) BaseCommand {
	return BaseCommand{Context: ctx, OptionSet: opts}
}

func (BaseCommand) Complete(args []string) error { return nil }

func MassageCommand(cmd *cobra.Command, names ...string) *cobra.Command {
	if cmd.Use == "" {
		cmd.Use = names[0] // SubCmdUse(name)
	} else {
		if !strings.HasSuffix(cmd.Use, names[0]+" ") {
			cmd.Use = names[0] + " " + cmd.Use
		}
	}
	if len(names) > 1 {
		cmd.Aliases = names[1:]
	}
	cmd.DisableFlagsInUseLine = true
	cmd.TraverseChildren = true
	return cmd
}

func SetupCommand(ocmcmd OCMCommand, names ...string) *cobra.Command {
	c := ocmcmd.ForName(names[0])
	MassageCommand(c, names...)
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if set, ok := ocmcmd.(options.OptionSetProvider); ok {
			set.AsOptionSet().ProcessOnOptions(options.CompleteOptionsWithCLIContext(ocmcmd))
		}
		err := ocmcmd.Complete(args)
		if err == nil {
			err = ocmcmd.Run()
		}
		if err != nil && ocmcmd.StdErr() != os.Stderr {
			out.Error(ocmcmd, err.Error())
		}
		return err
	}
	if u, ok := ocmcmd.(options.Usage); ok {
		c.Long = c.Long + u.Usage()
	}
	ocmcmd.AddFlags(c.Flags())
	return c
}

func Names(def []string, names ...string) []string {
	if len(names) == 0 {
		return def
	}
	return names
}
