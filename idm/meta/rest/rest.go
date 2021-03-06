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

package rest

import (
	"context"

	"github.com/emicklei/go-restful"
	"go.uber.org/zap"

	"github.com/micro/go-micro/errors"
	"github.com/pydio/cells/common"
	"github.com/pydio/cells/common/auth"
	"github.com/pydio/cells/common/auth/claim"
	"github.com/pydio/cells/common/log"
	"github.com/pydio/cells/common/proto/idm"
	"github.com/pydio/cells/common/proto/rest"
	"github.com/pydio/cells/common/proto/tree"
	"github.com/pydio/cells/common/service"
	"github.com/pydio/cells/common/service/defaults"
	serviceproto "github.com/pydio/cells/common/service/proto"
	"github.com/pydio/cells/common/service/resources"
	"github.com/pydio/cells/common/views"
	"github.com/pydio/cells/idm/meta/namespace"
)

func NewUserMetaHandler() *UserMetaHandler {
	handler := new(UserMetaHandler)
	handler.ServiceName = common.SERVICE_USER_META
	handler.ResourceName = "userMeta"
	handler.PoliciesLoader = handler.PoliciesForMeta
	return handler
}

type UserMetaHandler struct {
	resources.ResourceProviderHandler
}

// SwaggerTags list the names of the service tags declared in the swagger json implemented by this service
func (s *UserMetaHandler) SwaggerTags() []string {
	return []string{"UserMetaService"}
}

// Filter returns a function to filter the swagger path
func (s *UserMetaHandler) Filter() func(string) string {
	return nil
}

// Will check for namespace policies before updating / deleting
func (s *UserMetaHandler) UpdateUserMeta(req *restful.Request, rsp *restful.Response) {

	var input idm.UpdateUserMetaRequest
	if err := req.ReadEntity(&input); err != nil {
		service.RestError500(req, rsp, err)
		return
	}
	ctx := req.Request.Context()
	userMetaClient := idm.NewUserMetaServiceClient(common.SERVICE_GRPC_NAMESPACE_+common.SERVICE_USER_META, defaults.NewClient())
	nsList, e := s.ListAllNamespaces(ctx, userMetaClient)
	if e != nil {
		service.RestError500(req, rsp, e)
		return
	}
	var loadUuids []string
	// TODO: CHECK RIGHTS FOR NODE UUID WITH ROUTER ?

	// First check if the namespaces are globally accessible
	for _, meta := range input.MetaDatas {
		var ns *idm.UserMetaNamespace
		var exists bool
		if ns, exists = nsList[meta.Namespace]; !exists {
			service.RestError404(req, rsp, errors.NotFound(common.SERVICE_USER_META, "Namespace "+meta.Namespace+" is not defined!"))
			return
		}
		if !s.MatchPolicies(ctx, meta.Namespace, ns.Policies, serviceproto.ResourcePolicyAction_WRITE) {
			service.RestError403(req, rsp, errors.Forbidden(common.SERVICE_USER_META, "You are not authorized to write on namespace "+meta.Namespace))
			return
		}
		if meta.Uuid != "" {
			loadUuids = append(loadUuids, meta.Uuid)
		}
	}
	// Some existing meta will be updated / deleted : load their policies and check their rights!
	if len(loadUuids) > 0 {
		stream, e := userMetaClient.SearchUserMeta(ctx, &idm.SearchUserMetaRequest{MetaUuids: loadUuids})
		if e != nil {
			service.RestError500(req, rsp, e)
			return
		}
		defer stream.Close()
		for {
			resp, er := stream.Recv()
			if er != nil {
				break
			}
			if resp == nil {
				continue
			}
			if !s.MatchPolicies(ctx, resp.UserMeta.Uuid, resp.UserMeta.Policies, serviceproto.ResourcePolicyAction_WRITE) {
				service.RestError403(req, rsp, errors.Forbidden(common.SERVICE_USER_META, "You are not authorized to edit this meta "+resp.UserMeta.Namespace))
				return
			}
		}
	}
	if response, err := userMetaClient.UpdateUserMeta(ctx, &input); err != nil {
		service.RestError500(req, rsp, err)
	} else {
		rsp.WriteEntity(response)
	}

}

func (s *UserMetaHandler) SearchUserMeta(req *restful.Request, rsp *restful.Response) {

	var input idm.SearchUserMetaRequest
	if err := req.ReadEntity(&input); err != nil {
		service.RestError500(req, rsp, err)
		return
	}
	ctx := req.Request.Context()
	if output, e := s.PerformSearchMetaRequest(ctx, &input); e != nil {
		service.RestError500(req, rsp, e)
	} else {
		rsp.WriteEntity(output)
	}

}

// UserBookmarks searches meta with bookmark namespace and feeds a list of nodes with the results
func (s *UserMetaHandler) UserBookmarks(req *restful.Request, rsp *restful.Response) {

	searchRequest := &idm.SearchUserMetaRequest{
		Namespace: namespace.ReservedNamespaceBookmark,
	}
	router := views.NewUuidRouter(views.RouterOptions{})
	ctx := req.Request.Context()
	output, e := s.PerformSearchMetaRequest(ctx, searchRequest)
	if e != nil {
		service.RestError500(req, rsp, e)
		return
	}
	log.Logger(ctx).Info("Got Bookmarks : ", zap.Any("b", output))
	bulk := &rest.BulkMetaResponse{}
	for _, meta := range output.Metadatas {
		node := &tree.Node{
			Uuid: meta.NodeUuid,
		}
		if resp, e := router.ReadNode(ctx, &tree.ReadNodeRequest{Node: node}); e == nil {
			bulk.Nodes = append(bulk.Nodes, resp.Node.WithoutReservedMetas())
		} else {
			log.Logger(ctx).Error("ReadNode Error : ", zap.Error(e))
		}
	}
	log.Logger(ctx).Info("Return bulk : ", zap.Any("b", bulk))
	rsp.WriteEntity(bulk)

}

func (s *UserMetaHandler) UpdateUserMetaNamespace(req *restful.Request, rsp *restful.Response) {

	var input idm.UpdateUserMetaNamespaceRequest
	if err := req.ReadEntity(&input); err != nil {
		service.RestError500(req, rsp, err)
		return
	}
	ctx := req.Request.Context()
	if value := ctx.Value(claim.ContextKey); value != nil {
		claims := value.(claim.Claims)
		if claims.Profile != "admin" {
			service.RestError403(req, rsp, errors.Forbidden(common.SERVICE_USER_META, "You are not allowed to edit namespaces"))
			return
		}
	}

	nsClient := idm.NewUserMetaServiceClient(common.SERVICE_GRPC_NAMESPACE_+common.SERVICE_USER_META, defaults.NewClient())
	response, err := nsClient.UpdateUserMetaNamespace(ctx, &input)
	if err != nil {
		service.RestError500(req, rsp, err)
	} else {
		rsp.WriteEntity(response)
	}

}

func (s *UserMetaHandler) ListUserMetaNamespace(req *restful.Request, rsp *restful.Response) {

	nsClient := idm.NewUserMetaServiceClient(common.SERVICE_GRPC_NAMESPACE_+common.SERVICE_USER_META, defaults.NewClient())
	output := &rest.UserMetaNamespaceCollection{}
	if ns, err := s.ListAllNamespaces(req.Request.Context(), nsClient); err == nil {
		for _, n := range ns {
			if n.Namespace == namespace.ReservedNamespaceBookmark {
				continue
			}
			output.Namespaces = append(output.Namespaces, n)
		}
	}
	rsp.WriteEntity(output)

}

func (s *UserMetaHandler) PerformSearchMetaRequest(ctx context.Context, request *idm.SearchUserMetaRequest) (*rest.UserMetaCollection, error) {

	subjects, e := auth.SubjectsForResourcePolicyQuery(ctx, nil)
	if e != nil {
		return nil, e
	}
	// Append Subjects
	request.ResourceQuery = &serviceproto.ResourcePolicyQuery{
		Subjects: subjects,
	}

	userMetaClient := idm.NewUserMetaServiceClient(common.SERVICE_GRPC_NAMESPACE_+common.SERVICE_USER_META, defaults.NewClient())
	stream, er := userMetaClient.SearchUserMeta(ctx, request)
	if er != nil {
		return nil, e
	}
	output := &rest.UserMetaCollection{}
	defer stream.Close()
	for {
		resp, e := stream.Recv()
		if e != nil {
			break
		}
		if resp == nil {
			continue
		}
		output.Metadatas = append(output.Metadatas, resp.UserMeta)
	}

	return output, nil
}

func (s *UserMetaHandler) ListAllNamespaces(ctx context.Context, client idm.UserMetaServiceClient) (map[string]*idm.UserMetaNamespace, error) {

	stream, e := client.ListUserMetaNamespace(ctx, &idm.ListUserMetaNamespaceRequest{})
	if e != nil {
		return nil, e
	}
	result := make(map[string]*idm.UserMetaNamespace)
	defer stream.Close()
	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		if resp == nil {
			continue
		}
		result[resp.UserMetaNamespace.Namespace] = resp.UserMetaNamespace
	}
	return result, nil

}

func (s *UserMetaHandler) PoliciesForMeta(ctx context.Context, resourceId string, resourceClient interface{}) (policies []*serviceproto.ResourcePolicy, e error) {

	return
}
