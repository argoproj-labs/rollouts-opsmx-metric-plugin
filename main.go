/*
 * Copyright 2023 OpsMx, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"github.com/argoproj-labs/rollouts-opsmx-metric-plugin/internal/plugin"
	rolloutsPlugin "github.com/argoproj/argo-rollouts/metricproviders/plugin/rpc"
	goPlugin "github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
)

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var handshakeConfig = goPlugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARGO_ROLLOUTS_RPC_PLUGIN",
	MagicCookieValue: "metricprovider",
}

func main() {
	logCtx := *log.WithFields(log.Fields{"plugin": "opsmx"})

	rpcPluginImp := &plugin.RpcPlugin{
		LogCtx: logCtx,
	}
	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]goPlugin.Plugin{
		"RpcMetricProviderPlugin": &rolloutsPlugin.RpcMetricProviderPlugin{Impl: rpcPluginImp},
	}

	goPlugin.Serve(&goPlugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
	})
}
