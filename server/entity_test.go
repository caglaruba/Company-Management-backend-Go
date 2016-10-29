package server_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"git.simplendi.com/FirmQ/frontend-server/server"
	grpc_gateway_entity "git.simplendi.com/FirmQ/frontend-server/server/proto/entity"
	//"github.com/golang/glog"
	"github.com/golang/protobuf/jsonpb"
	. "gopkg.in/check.v1"
	"net/http"
	"time"
)

type EntityTestSuite struct {
	server *server.Server
}

var _ = Suite(&EntityTestSuite{})

func (s *EntityTestSuite) SetUpSuite(c *C) {
	var err error
	flag.Parse()

	cfg := &server.Config{
		EmailConfirmationTTL: time.Second * 5,
		SMSConfirmationTTL:   time.Second * 2,
	}
	s.server, err = server.NewServer(cfg)

	c.Assert(err, IsNil)

	go s.server.RunServer()
	time.Sleep(time.Second * 4)
}

func createTestEntity(companyId, token string) (*grpc_gateway_entity.Entity, error) {
	entityName := fmt.Sprintf("entityName_%v", time.Now().UnixNano())

	url := "http://127.0.0.1:8080/v1/entity"

	entity := grpc_gateway_entity.Entity{
		CommonName: entityName,
		CompanyId:   companyId,
	}

	entityTxt, _ := json.Marshal(entity)
	req, err := http.NewRequest("POST", url, bytes.NewReader(entityTxt))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	createdEntity := server.NewEntityResponse()

	err = jsonpb.Unmarshal(resp.Body, createdEntity)
	return createdEntity.Data, nil
}

// create entity by user with non-admin permissions
func (m *EntityTestSuite) TestCreateByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	createdUser, err := createTestUser(email, token, "809044", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	entityName := fmt.Sprintf("entityName_%v", time.Now().UnixNano())

	//try to create new entity with credentials of non-admin entity
	entity := grpc_gateway_entity.Entity{
		CommonName: entityName,
	}

	entityTxt, _ := json.Marshal(entity)
	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v1/entity", bytes.NewReader(entityTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	message := server.NewEntityResponse()
	err = jsonpb.Unmarshal(resp.Body, message)
	c.Assert(err, IsNil)

	c.Assert(message.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(message.Meta.Ok, Equals, true)
	c.Assert(message.Meta.Error, Equals, "")
	c.Assert(message.Data.CompanyId, Equals, createdUser.CompanyId)
	c.Assert(message.Data.CreatedBy, Equals, createdUser.Id)
	c.Assert(message.Data.Latest, Equals, true)
}

// create entity by user with admin permissions
func (m *EntityTestSuite) TestCreateByAdmin(c *C) {
	token := getTestDefaultAuthToken()

	entityName := fmt.Sprintf("entityName_%v", time.Now().UnixNano())

	//try to create new entity with credentials of non-admin entity
	entity := grpc_gateway_entity.Entity{
		CommonName: entityName,
	}

	entityTxt, _ := json.Marshal(entity)
	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v1/entity", bytes.NewReader(entityTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	message := server.NewEntityResponse()
	err = jsonpb.Unmarshal(resp.Body, message)
	c.Assert(err, IsNil)

	c.Assert(message.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(message.Meta.Ok, Equals, false)
	c.Assert(message.Meta.Error, Equals, server.ErrMissedRequiredField.Error())
}

// get entity by user with admin permissions
func (m *EntityTestSuite) TestGetByAdminPermissions(c *C) {
	token := getTestDefaultAuthToken()
	companyId := fmt.Sprintf("company_%v", time.Now().UnixNano())

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	_, err := createTestUser(email, token, companyId, false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))
	_, err = createTestEntity(companyId, createdUserToken)
	c.Assert(err, IsNil)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/entity/%v", companyId), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	retrievedEntity := server.NewEntityResponse()
	err = jsonpb.Unmarshal(resp.Body, retrievedEntity)
	c.Assert(err, IsNil)

	c.Assert(retrievedEntity.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(retrievedEntity.Meta.Ok, Equals, false)
	c.Assert(retrievedEntity.Meta.Error, Equals, server.ErrMissedRequiredField.Error())
}

// get entity by user with non-admin permissions
func (m *EntityTestSuite) TestGetByNonAdminPermissions(c *C) {
	token := getTestDefaultAuthToken()

	companyId := fmt.Sprintf("company_%v", time.Now().UnixNano())

	// try to retrieve entity by company user
	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err := createTestUser(email, token, companyId, false)
	c.Assert(err, IsNil)
	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	entity, err := createTestEntity(companyId, createdUserToken)
	c.Assert(err, IsNil)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/entity/%v", entity.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	retrievedEntity := server.NewEntityResponse()
	err = jsonpb.Unmarshal(resp.Body, retrievedEntity)
	c.Assert(err, IsNil)

	c.Assert(retrievedEntity.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(retrievedEntity.Data.Id, Equals, entity.Id)
	c.Assert(retrievedEntity.Data.CommonName, Equals, entity.CommonName)

	// try to retrieve entity by non-company user
	email = fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err = createTestUser(email, token, "352", false)
	c.Assert(err, IsNil)
	createdUserToken = getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/entity/%v", entity.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	resp, err = client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	retrievedEntity = server.NewEntityResponse()
	err = jsonpb.Unmarshal(resp.Body, retrievedEntity)
	c.Assert(err, IsNil)

	c.Assert(retrievedEntity.Meta.StatusCode, Equals, HttpStatusNotFound)
	c.Assert(retrievedEntity.Data.Id, Equals, "")
}

// update entity by user by admin permissions
func (m *EntityTestSuite) TestUpdateByAdmin(c *C) {
	token := getTestDefaultAuthToken()

	companyId := fmt.Sprintf("company_%v", time.Now().UnixNano())

	// try to retrieve entity by non-company user
	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err := createTestUser(email, token, "352", false)
	c.Assert(err, IsNil)
	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	entity, err := createTestEntity(companyId, createdUserToken)

	entity.CommonName = fmt.Sprintf("updatedEntityName_%v", time.Now().UnixNano())

	entityTxt, _ := json.Marshal(entity)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/entity/%v", entity.Id), bytes.NewReader(entityTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	message := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, message)
	c.Assert(err, IsNil)

	c.Assert(message.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(message.Meta.Ok, Equals, false)
	c.Assert(message.Meta.Error, Equals, server.ErrMissedRequiredField.Error())
}

// update entity by user by non-admin permissions
func (m *EntityTestSuite) TestUpdateByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	// try to retrieve entity by company user
	companyId := fmt.Sprintf("company_%v", time.Now().UnixNano())
	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err := createTestUser(email, token, companyId, false)
	c.Assert(err, IsNil)
	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	email = fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err = createTestUser(email, token, companyId+"qwfwf", false)
	c.Assert(err, IsNil)
	createdUserToken2 := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	entity, err := createTestEntity(companyId, createdUserToken)

	entityTxt, _ := json.Marshal(entity)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/entity/%v", entity.Id), bytes.NewReader(entityTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken2)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	message := server.NewEntityResponse()
	err = jsonpb.Unmarshal(resp.Body, message)
	c.Assert(err, IsNil)

	c.Assert(message.Meta.StatusCode, Equals, HttpStatusNotFound)
	c.Assert(message.Meta.Ok, Equals, false)
	c.Assert(message.Meta.Error, Not(Equals), "")
}

// get all entity revisions by admin
func (m *EntityTestSuite) TestGetAllRevsByAdmin(c *C) {
	token := getTestDefaultAuthToken()

	companyId := fmt.Sprintf("company_%v", time.Now().UnixNano())
	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err := createTestUser(email, token, companyId, false)
	c.Assert(err, IsNil)
	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	entity, err := createTestEntity(companyId, createdUserToken)

	entity.CommonName = fmt.Sprintf("updatedEntityName_%v", time.Now().UnixNano())

	// update entity
	entityTxt, _ := json.Marshal(entity)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/entity/%v", entity.Id), bytes.NewReader(entityTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	message := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, message)
	c.Assert(err, IsNil)

	c.Assert(message.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(message.Meta.Ok, Equals, false)
	c.Assert(message.Meta.Error, Equals, server.ErrMissedRequiredField.Error())
}

// get all entity revisions by non-admin
func (m *EntityTestSuite) TestGetAllRevsByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	companyId := fmt.Sprintf("company_%v", time.Now().UnixNano())

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err := createTestUser(email, token, companyId, false)
	c.Assert(err, IsNil)
	createdUserToken1 := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	entity, err := createTestEntity(companyId, createdUserToken1)

	entity.CommonName = fmt.Sprintf("updatedEntityName_%v", time.Now().UnixNano())

	// update entity
	entityTxt, _ := json.Marshal(entity)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/entity/%v", entity.Id), bytes.NewReader(entityTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken1)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	message := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, message)
	c.Assert(err, IsNil)

	c.Assert(message.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(message.Meta.Ok, Equals, true)
	c.Assert(message.Meta.Error, Equals, "")

	// get updated entity
	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/entity_revs/%v", entity.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken1)

	resp, err = client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	retrievedEntities := grpc_gateway_entity.EntityListResponse{}
	err = jsonpb.Unmarshal(resp.Body, &retrievedEntities)
	c.Assert(err, IsNil)

	c.Assert(retrievedEntities.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(len(retrievedEntities.Data), Equals, 2)

	orderRevs := 0

	for _, en := range retrievedEntities.Data {
		if en.Rev == 0 || en.Rev == 1 {
			orderRevs += 1
		}
	}

	c.Assert(orderRevs, Equals, 2)

	// get updated entity by non-admin user and non-company user
	email = fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err = createTestUser(email, token, "12341234", false)
	c.Assert(err, IsNil)
	createdUserToken2 := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/entity_revs/%v", entity.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken2)

	resp, err = client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	list := server.NewEntityListResponse()
	err = jsonpb.Unmarshal(resp.Body, list)
	c.Assert(err, IsNil)
	c.Assert(list.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(len(list.Data), Equals, 0)
}
