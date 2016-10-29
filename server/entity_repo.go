package server

import (
	"errors"
	grpc_gateway_entity "git.simplendi.com/FirmQ/frontend-server/server/proto/entity"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// ErrMissedRequiredField - error when cannot find required field
var ErrMissedRequiredField = errors.New("cannot find required field")

// EntityRepo - model for accessing entitys in database
type EntityRepo struct {
	sess *mgo.Database
	coll string
}

// NewEntityRepo - returns new instance of EntityRepo which provide access to entity model
func NewEntityRepo(sess *mgo.Database) *EntityRepo {
	return &EntityRepo{
		sess: sess,
		coll: "entities",
	}
}

// CreateEntity - create new entity
func (ur *EntityRepo) CreateEntity(entity *grpc_gateway_entity.Entity) (*grpc_gateway_entity.Entity, error) {
	c := ur.sess.C(ur.coll)

	return entity, c.Insert(entity)
}

// GetLatestEntity - get entity from database by id
func (ur *EntityRepo) GetLatestEntity(id, companyID string) (*grpc_gateway_entity.Entity, error) {
	c := ur.sess.C(ur.coll)
	ent := grpc_gateway_entity.Entity{}
	err := c.Find(bson.M{"id": id, "companyid": companyID, "latest": true}).One(&ent)
	return &ent, err
}

// GetEntityRevs - get entity revisions from database by id
func (ur *EntityRepo) GetEntityRevs(id, companyID string) (*grpc_gateway_entity.EntityListResponse, error) {
	var err error
	c := ur.sess.C(ur.coll)
	entities := NewEntityListResponse()
	err = c.Find(bson.M{"id": id, "companyid": companyID}).Sort("-rev").All(&entities.Data)
	return entities, err
}

// GetEntities - get entities from database
func (ur *EntityRepo) GetEntities(companyID string, params *grpc_gateway_entity.EntityListRequest) (*grpc_gateway_entity.EntityListResponse, error) {
	c := ur.sess.C(ur.coll)
	entities := NewEntityListResponse()
	var err error

	mgoParams := bson.M{
		"latest": true,
	}

	if params.Type != "" {
		mgoParams["type"] = params.Type
	}

	if companyID == "" {
		err = c.Find(mgoParams).All(&entities.Data)
	} else {
		mgoParams["companyid"] = companyID
		err = c.Find(mgoParams).All(&entities.Data)
	}
	return entities, err
}

// CreateIndexes - create necessary indexes for fast executing
func (ur *EntityRepo) CreateIndexes() {
	c := ur.sess.C(ur.coll)
	c.EnsureIndex(mgo.Index{
		Key:    []string{"id"},
		Unique: true,
	})
	c.EnsureIndex(mgo.Index{
		Key:    []string{"rev"},
		Unique: true,
	})
}

// DeleteEntityByID - set entity as disabled from database by id
func (ur *EntityRepo) DeleteEntityByID(id string) error {
	c := ur.sess.C(ur.coll)
	err := c.Update(bson.M{"id": id}, bson.M{"$set": bson.M{"isenabled": false}})
	return err
}

// UpdateEntity - update entity info by id
func (ur *EntityRepo) UpdateEntity(entity *grpc_gateway_entity.Entity) (*grpc_gateway_entity.Entity, error) {
	c := ur.sess.C(ur.coll)

	oldEntity, err := ur.GetLatestEntity(entity.Id, entity.CompanyId)
	if err != nil {
		return nil, err
	}

	entity.Rev = oldEntity.Rev + 1
	if err := c.Insert(entity); err != nil {
		return nil, err
	}

	return entity, c.Update(bson.M{"id": oldEntity.Id, "createdat": oldEntity.CreatedAt}, bson.M{"$set": bson.M{"latest": false}})
}
