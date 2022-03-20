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
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// OCMCommand is a command pattern, thta can be instantiated for a dediated
// sub command name.
type OCMCommand interface {
	ForName(name string) *cobra.Command
	AddFlags(fs *pflag.FlagSet)
	Complete(args []string) error
	Run() error
}

func SetupCommand(ocmcmd OCMCommand, names ...string) *cobra.Command {
	c := ocmcmd.ForName(names[0])
	if !strings.HasSuffix(c.Use, names[0]+" ") {
		c.Use = names[0] + " " + c.Use
	}
	c.Aliases = names[1:]
	c.Run = func(cmd *cobra.Command, args []string) {
		if err := ocmcmd.Complete(args); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if err := ocmcmd.Run(); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}
	c.TraverseChildren = true
	ocmcmd.AddFlags(c.Flags())
	return c
}