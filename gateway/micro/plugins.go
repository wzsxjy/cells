/*
 * Copyright (c) 2018. Abstrium SAS <team (at) pydio.com>
 * This file is part of Pydio Cells.
 *
 * Pydio Cells is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Pydio Cells is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Pydio Cells.  If not, see <http://www.gnu.org/licenses/>.
 *
 * The latest code can be found at <https://pydio.com>.
 */

// Package micro starts a micro web service in API mode to dispatch all REST calls to the underlying services
package micro

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/micro/cli"
	"github.com/micro/go-micro/cmd"
	"github.com/micro/micro/api"
	"github.com/pydio/cells/common"
	"github.com/pydio/cells/common/service"
	"github.com/pydio/cells/common/service/defaults"

	"github.com/pydio/cells/common/config"
	"github.com/pydio/cells/common/log"
)

func init() {
	service.NewService(
		service.Name(common.SERVICE_MICRO_API),
		service.Tag(common.SERVICE_TAG_GATEWAY),
		service.Description("Proxy handler to dispatch REST requests to the underlying services"),
		service.WithGeneric(func(ctx context.Context, cancel context.CancelFunc) (service.Runner, service.Checker, service.Stopper, error) {
			port := config.Get("ports", common.SERVICE_MICRO_API).Int(0)
			flagSet := flag.NewFlagSet("test", flag.ExitOnError)

			if config.Get("cert", "http", "ssl").Bool(false) {
				log.Logger(ctx).Info("MICRO WEB SHOULD START WITH SSL")
				certFile := config.Get("cert", "http", "certFile").String("")
				keyFile := config.Get("cert", "http", "keyFile").String("")
				flagSet.Bool("enable_tls", true, "")
				flagSet.String("tls_cert_file", certFile, "")
				flagSet.String("tls_key_file", keyFile, "")
			}

			api.Handler = "proxy"
			api.Name = common.SERVICE_MICRO_API
			api.Address = fmt.Sprintf(":%d", port)
			api.Namespace = strings.TrimRight(common.SERVICE_REST_NAMESPACE_, ".")
			api.CORS = map[string]bool{"*": true}

			app := cli.NewApp()

			c := defaults.NewClient()
			s := defaults.NewServer()
			r := defaults.Registry()
			b := defaults.Broker()
			t := defaults.Transport()

			return service.RunnerFunc(func() error {
					cmd.Init(
						cmd.Client(&c),
						cmd.Server(&s),
						cmd.Registry(&r),
						cmd.Broker(&b),
						cmd.Transport(&t),
					)
					api.Commands()[0].Action(cli.NewContext(app, flagSet, nil))
					return nil
				}), service.CheckerFunc(func() error {
					return nil
				}), service.StopperFunc(func() error {
					return nil
				}), nil
		}),
	)
}
