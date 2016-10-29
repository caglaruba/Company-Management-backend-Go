package server

import (
	grpc_gateway_common "git.simplendi.com/FirmQ/frontend-server/server/proto/common"
	grpc_gateway_entity "git.simplendi.com/FirmQ/frontend-server/server/proto/entity"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"net/http"
	"time"
)

type entityServer struct {
	sess *mgo.Session
}

// NewEntityResponse - create new instance of entity response
func NewEntityResponse() *grpc_gateway_entity.EntityResponse {
	message := &grpc_gateway_entity.EntityResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	message.Data = &grpc_gateway_entity.Entity{}
	return message
}

// NewEntityListResponse - create new instance of entity list response
func NewEntityListResponse() *grpc_gateway_entity.EntityListResponse {
	message := &grpc_gateway_entity.EntityListResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	return message
}

// NewEntityServer - returns new grpc server which provide entity-related functionality
func NewEntityServer() grpc_gateway_entity.EntityServiceServer {
	return new(entityServer)
}

func (es *entityServer) CreateEntity(ctx context.Context, entity *grpc_gateway_entity.Entity) (*grpc_gateway_entity.EntityResponse, error) {
	message := NewEntityResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	currentUser, _ := GetCurrentUser(ctx)

	if currentUser.CompanyId == "" {
		message.Meta.Ok = false
		message.Meta.Error = ErrMissedRequiredField.Error()
		return message, nil
	}

	entity.CompanyId = currentUser.CompanyId
	entity.CreatedBy = currentUser.Id
	entity.CreatedAt = time.Now().Unix()
	entity.Id = uuid.NewV4().String()
	entity.Rev = 0
	entity.Latest = true

	entityRepo := NewEntityRepo(sess)
	createdEntity, err := entityRepo.CreateEntity(entity)

	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
	} else {
		message.Meta.Ok = true
		message.Data = createdEntity
	}

	return message, nil
}

func (es *entityServer) UpdateEntity(ctx context.Context, entity *grpc_gateway_entity.Entity) (*grpc_gateway_entity.EntityResponse, error) {
	message := NewEntityResponse()
	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	currentUser, _ := GetCurrentUser(ctx)

	if currentUser.CompanyId == "" {
		message.Meta.Ok = false
		message.Meta.Error = ErrMissedRequiredField.Error()
		return message, nil
	}

	entity.CompanyId = currentUser.CompanyId
	entity.CreatedBy = currentUser.Id
	entity.CreatedAt = time.Now().Unix()
	entity.Latest = true

	entityRepo := NewEntityRepo(sess)
	message.Data, err = entityRepo.UpdateEntity(entity)

	if err != nil {
		if err == mgo.ErrNotFound {
			message.Meta.StatusCode = http.StatusNotFound
		}

		message.Meta.Ok = false
		message.Meta.Error = err.Error()
	} else {
		message.Meta.Ok = true
	}

	return message, nil
}

func (es *entityServer) GetLatestEntity(ctx context.Context, in *grpc_gateway_common.IDRequest) (*grpc_gateway_entity.EntityResponse, error) {
	entity := NewEntityResponse()

	currentUser, err := GetCurrentUser(ctx)
	if err != nil {
		entity.Meta.Ok = false
		entity.Meta.Error = err.Error()
		return entity, nil
	}

	if currentUser.CompanyId == "" {
		entity.Meta.Ok = false
		entity.Meta.Error = ErrMissedRequiredField.Error()
		return entity, nil
	}

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		entity.Meta.Ok = false
		entity.Meta.Error = err.Error()
		return entity, nil
	}

	companyID := ""
	if !currentUser.IsAdmin {
		companyID = currentUser.CompanyId
	}

	entityRepo := NewEntityRepo(sess)
	entity.Data, err = entityRepo.GetLatestEntity(in.Id, companyID)
	if err != nil {
		if err == mgo.ErrNotFound {
			entity.Meta.StatusCode = http.StatusNotFound
		}

		entity.Meta.Ok = false
		entity.Meta.Error = err.Error()
	} else {
		entity.Meta.Ok = true
	}

	if entity.Data.Directors == nil {
		entity.Data.Directors = &grpc_gateway_entity.EntityLink{EntityId: ""}
	}
	if entity.Data.Proxyholders == nil {
		entity.Data.Proxyholders = &grpc_gateway_entity.EntityLink{EntityId: ""}
	}
	if entity.Data.Trustees == nil {
		entity.Data.Trustees = &grpc_gateway_entity.EntityLink{EntityId: ""}
	}
	if entity.Data.Shareholders == nil {
		entity.Data.Shareholders = &grpc_gateway_entity.EntityLink{EntityId: ""}
	}
	return entity, nil
}

func (es *entityServer) GetEntityRevisions(ctx context.Context, in *grpc_gateway_common.IDRequest) (*grpc_gateway_entity.EntityListResponse, error) {
	entityList := NewEntityListResponse()

	currentUser, err := GetCurrentUser(ctx)
	if err != nil {
		entityList.Meta.Ok = false
		entityList.Meta.Error = err.Error()
		return entityList, nil
	}

	if currentUser.CompanyId == "" {
		entityList.Meta.Ok = false
		entityList.Meta.Error = ErrMissedRequiredField.Error()
		return entityList, nil
	}

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		entityList.Meta.Ok = false
		entityList.Meta.Error = err.Error()
		return entityList, nil
	}

	companyID := ""
	if !currentUser.IsAdmin {
		companyID = currentUser.CompanyId
	}

	entityRepo := NewEntityRepo(sess)
	entityList, err = entityRepo.GetEntityRevs(in.Id, companyID)

	// Retrieve users information
	createdBys := []string{}
	for _, entity := range entityList.Data {
		isFound := false
		for _, createdBy := range createdBys {
			if createdBy == entity.CreatedBy {
				isFound = true
				break
			}
		}

		if !isFound {
			createdBys = append(createdBys, entity.CreatedBy)
		}
	}

	if err != nil {
		if err == mgo.ErrNotFound {
			entityList.Meta.StatusCode = http.StatusNotFound
		}

		entityList.Meta.Ok = false
		entityList.Meta.Error = err.Error()
	} else {
		entityList.Meta.Ok = true
	}
	return entityList, nil
}

func (es *entityServer) GetEntities(ctx context.Context, in *grpc_gateway_entity.EntityListRequest) (*grpc_gateway_entity.EntityListResponse, error) {
	entityList := NewEntityListResponse()

	currentUser, err := GetCurrentUser(ctx)
	if err != nil {
		entityList.Meta.Ok = false
		entityList.Meta.Error = err.Error()
		return entityList, nil
	}

	if currentUser.CompanyId == "" {
		entityList.Meta.Ok = false
		entityList.Meta.Error = ErrMissedRequiredField.Error()
		return entityList, nil
	}

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		entityList.Meta.Ok = false
		entityList.Meta.Error = err.Error()
		return entityList, nil
	}

	entityRepo := NewEntityRepo(sess)
	entityList, err = entityRepo.GetEntities(currentUser.CompanyId, in)

	if err != nil {
		entityList.Meta.Ok = false
		entityList.Meta.Error = err.Error()
	} else {
		entityList.Meta.Ok = true
	}
	return entityList, nil
}
