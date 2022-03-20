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

package get

import (
	"github.com/gardener/ocm/cmds/ocm/clictx"
	artefacts "github.com/gardener/ocm/cmds/ocm/commands/ocicmds/artefacts/get"
	components "github.com/gardener/ocm/cmds/ocm/commands/ocmcmds/components/get"
	"github.com/spf13/cobra"
)

// NewCommand creates a new command.
func NewCommand(ctx clictx.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:              "get",
		TraverseChildren: true,
	}
	cmd.AddCommand(artefacts.NewCommand(ctx, "artefacts", "artefact", "art", "a"))
	cmd.AddCommand(components.NewCommand(ctx, "components", "component", "comps", "comp", "c"))
	return cmd
}
