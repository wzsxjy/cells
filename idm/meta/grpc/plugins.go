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

// Package grpc provides persistence layer for user-defined metadata
package grpc

import (
	"github.com/micro/go-micro"

	"github.com/pydio/cells/common"
	"github.com/pydio/cells/common/proto/idm"
	"github.com/pydio/cells/common/proto/tree"
	"github.com/pydio/cells/common/service"
	"github.com/pydio/cells/common/service/context"
	"github.com/pydio/cells/idm/meta"
)

func init() {
	service.NewService(
		service.Name(common.SERVICE_GRPC_NAMESPACE_+common.SERVICE_USER_META),
		service.Tag(common.SERVICE_TAG_IDM),
		service.Description("User-defined Metadata"),
		service.WithStorage(meta.NewDAO, "idm_usr_meta"),
		service.WithMicro(func(m micro.Service) error {
			ctx := m.Options().Context
			server := new(Handler)
			m.Init(micro.Metadata(map[string]string{"MetaProvider": "stream"}))

			idm.RegisterUserMetaServiceHandler(m.Options().Server, server)
			tree.RegisterNodeProviderStreamerHandler(m.Options().Server, server)

			// Clean role on user deletion
			cleaner := NewCleaner(server, servicecontext.GetDAO(ctx))
			if err := m.Options().Server.Subscribe(m.Options().Server.NewSubscriber(common.TOPIC_IDM_EVENT, cleaner)); err != nil {
				return err
			}
			return nil
		}),
	)
}
