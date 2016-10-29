package server_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"git.simplendi.com/FirmQ/frontend-server/server"
	grpc_gateway_company "git.simplendi.com/FirmQ/frontend-server/server/proto/company"
	//"github.com/golang/glog"
	"github.com/golang/protobuf/jsonpb"
	. "gopkg.in/check.v1"
	"net/http"
	"time"
)

type CompanyTestSuite struct {
	server *server.Server
}

var _ = Suite(&CompanyTestSuite{})

func (ct *CompanyTestSuite) SetUpSuite(c *C) {
	var err error
	flag.Parse()
	cfg := &server.Config{
		EmailConfirmationTTL: time.Second * 5,
		SMSConfirmationTTL:   time.Second * 2,
	}
	ct.server, err = server.NewServer(cfg)
	if err != nil {
		panic(err)
	}

	go ct.server.RunServer()
	time.Sleep(time.Second * 4)
}

func createTestCompany(companyName, token string) (*grpc_gateway_company.Company, error) {
	url := "http://127.0.0.1:8080/v1/company"

	company := grpc_gateway_company.Company{
		Name:      companyName,
		IsEnabled: true,
	}

	companyTxt, _ := json.Marshal(company)
	req, err := http.NewRequest("POST", url, bytes.NewReader(companyTxt))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	companyResp := server.NewIDResponse()
	err = jsonpb.Unmarshal(resp.Body, companyResp)
	if err != nil {
		return nil, err
	}

	company.Id = companyResp.Id
	return &company, nil
}

// create company by user with non-admin permissions
func (ct *CompanyTestSuite) TestCreateByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	_, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	companyName := fmt.Sprintf("companyName_%v", time.Now().UnixNano())

	//try to create new company with credentials of non-admin company
	company := grpc_gateway_company.Company{
		Name: companyName,
	}

	companyTxt, _ := json.Marshal(company)
	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v1/company", bytes.NewReader(companyTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	createResp := server.NewIDResponse()
	err = jsonpb.Unmarshal(resp.Body, createResp)
	c.Assert(err, IsNil)

	c.Assert(createResp.Meta.StatusCode, Equals, HttpStatusForbidden)
}

// create company by user with admin permissions
func (ct *CompanyTestSuite) TestCreateByAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	_, err := createTestUser(email, token, "", true)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	//try to create new company with credentials of admin company
	companyName := fmt.Sprintf("companyName_%v", time.Now().UnixNano())
	company := grpc_gateway_company.Company{
		Name: companyName,
	}

	userTxt, _ := json.Marshal(company)
	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v1/company", bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	idResp := server.NewIDResponse()
	err = jsonpb.Unmarshal(resp.Body, idResp)
	c.Assert(err, IsNil)

	c.Assert(idResp.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(idResp.Id, Not(Equals), "")
}

// get company by user with admin permissions
func (ct *CompanyTestSuite) TestGet(c *C) {
	token := getTestDefaultAuthToken()
	companyName := fmt.Sprintf("companyName_%v", time.Now().UnixNano())

	company, err := createTestCompany(companyName, token)
	c.Assert(err, IsNil)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	retrievedCompany := server.NewCompanyResponse()
	err = jsonpb.Unmarshal(resp.Body, retrievedCompany)
	c.Assert(err, IsNil)

	c.Assert(retrievedCompany.Meta.Ok, Equals, true)
	c.Assert(retrievedCompany.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(retrievedCompany.Data.Id, Equals, company.Id)
	c.Assert(retrievedCompany.Data.Name, Equals, company.Name)
}

// create company by user with admin permissions
func (ct *CompanyTestSuite) TestGetForNonAdminUser(c *C) {
	token := getTestDefaultAuthToken()

	companyName := fmt.Sprintf("companyName_%v", time.Now().UnixNano())

	company, err := createTestCompany(companyName, token)
	c.Assert(err, IsNil)

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err = createTestUser(email, token, "", false)
	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	anotherEmail := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err = createTestUser(anotherEmail, token, company.Id, false)
	anotherCreatedUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, anotherEmail))

	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	companyResp := server.NewCompanyResponse()

	err = jsonpb.Unmarshal(resp.Body, companyResp)
	c.Assert(err, IsNil)

	c.Assert(companyResp.Meta.StatusCode, Equals, HttpStatusForbidden)

	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", anotherCreatedUserToken)

	resp, err = client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	err = jsonpb.Unmarshal(resp.Body, companyResp)
	c.Assert(err, IsNil)

	c.Assert(companyResp.Meta.StatusCode, Equals, HttpStatusOK)
}

// get companies
func (ct *CompanyTestSuite) TestGetCompanies(c *C) {
	token := getTestDefaultAuthToken()

	companyName1 := fmt.Sprintf("companyName_%v", time.Now().UnixNano())
	company1, err := createTestCompany(companyName1, token)
	c.Assert(err, IsNil)

	companyName2 := fmt.Sprintf("companyName_%v", time.Now().UnixNano())
	company2, err := createTestCompany(companyName2, token)
	c.Assert(err, IsNil)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/company"), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	retrievedCompanies := server.NewCompanyListResponse()
	err = jsonpb.Unmarshal(resp.Body, retrievedCompanies)
	c.Assert(err, IsNil)

	c.Assert(retrievedCompanies.Meta.Ok, Equals, true)
	c.Assert(retrievedCompanies.Meta.StatusCode, Equals, HttpStatusOK)

	cnt := 2
	for _, comp := range retrievedCompanies.Data {
		if comp.Id == company1.Id || comp.Id == company2.Id {
			cnt -= 1
		}
	}

	c.Assert(cnt, Equals, 0)
}

// get companies by non-admin user
func (ct *CompanyTestSuite) TestGetCompaniesByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err := createTestUser(email, token, "", false)
	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/company"), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	companyListResponse := server.NewCompanyListResponse()
	err = jsonpb.Unmarshal(resp.Body, companyListResponse)
	c.Assert(err, IsNil)

	c.Assert(companyListResponse.Meta.StatusCode, Equals, HttpStatusForbidden)
}

// delete company by user with admin permissions
func (ct *CompanyTestSuite) TestDelete(c *C) {
	token := getTestDefaultAuthToken()

	// create new another company
	companyName := fmt.Sprintf("companyName_%v", time.Now().UnixNano())

	company, err := createTestCompany(companyName, token)
	c.Assert(err, IsNil)

	// try to remove company
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	mess := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, mess)
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(mess.Meta.Ok, Equals, true)
	c.Assert(mess.Meta.Error, Equals, "")

	// try get removed company
	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	resp, err = client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	companyResponse := server.NewCompanyResponse()
	err = jsonpb.Unmarshal(resp.Body, companyResponse)
	c.Assert(err, IsNil)
	c.Assert(companyResponse.Meta.StatusCode, Equals, HttpStatusNotFound)

	// try get removed company
	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/company"), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	resp, err = client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	companyListResponse := server.NewCompanyListResponse()
	err = jsonpb.Unmarshal(resp.Body, companyListResponse)
	c.Assert(err, IsNil)

	c.Assert(companyListResponse.Meta.StatusCode, Equals, HttpStatusOK)

	isFound := false
	for _, comp := range companyListResponse.Data {
		if comp.Id == company.Id {
			isFound = true
			break
		}
	}

	c.Assert(isFound, Equals, false)
}

// delete company by user with non-admin permissions
func (ct *CompanyTestSuite) TestDeleteByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	// create new another company
	companyName := fmt.Sprintf("companyName_%v", time.Now().UnixNano())

	company, err := createTestCompany(companyName, token)
	c.Assert(err, IsNil)

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err = createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	// try to remove company
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()
	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	cmResp := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, cmResp)
	c.Assert(err, IsNil)
	c.Assert(cmResp.Meta.StatusCode, Equals, HttpStatusForbidden)
}

// update company by user with admin permissions
func (ct *CompanyTestSuite) TestUpdate(c *C) {
	token := getTestDefaultAuthToken()

	// create new company
	companyName := fmt.Sprintf("companyName_%v", time.Now().UnixNano())

	company, err := createTestCompany(companyName, token)
	c.Assert(err, IsNil)

	anotherCompanyName := fmt.Sprintf("anotherCompanyName_%v", time.Now().UnixNano())

	company.Name = anotherCompanyName
	company.IsEnabled = false

	companyTxt, _ := json.Marshal(company)

	// try to update company
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), bytes.NewReader(companyTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	mess := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, mess)
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(mess.Meta.Ok, Equals, true)
	c.Assert(mess.Meta.Error, Equals, "")

	// try get updated company
	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	resp, err = client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	retrievedCompany := server.NewCompanyResponse()
	err = jsonpb.Unmarshal(resp.Body, retrievedCompany)
	c.Assert(err, IsNil)

	c.Assert(retrievedCompany.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(retrievedCompany.Data.Name, Equals, company.Name)
	c.Assert(retrievedCompany.Data.IsEnabled, Not(Equals), company.IsEnabled)
}

// update company by user with non-admin permissions
func (ct *CompanyTestSuite) TestUpdateByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	// create new another company
	companyName := fmt.Sprintf("companyName_%v", time.Now().UnixNano())

	company, err := createTestCompany(companyName, token)
	c.Assert(err, IsNil)

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err = createTestUser(email, token, "", false)
	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	// try to update company
	companyTxt, _ := json.Marshal(company)

	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/company/%v", company.Id), bytes.NewReader(companyTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()
	resp, err := client.Do(req)
	defer resp.Body.Close()
	c.Assert(err, IsNil)

	idResp := server.NewIDResponse()
	err = jsonpb.Unmarshal(resp.Body, idResp)
	c.Assert(err, IsNil)
	c.Assert(idResp.Meta.StatusCode, Equals, HttpStatusForbidden)
}
