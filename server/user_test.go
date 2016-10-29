package server_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"git.simplendi.com/FirmQ/frontend-server/server"
	grpc_gateway_user "git.simplendi.com/FirmQ/frontend-server/server/proto/user"
	"github.com/golang/glog"
	"github.com/golang/protobuf/jsonpb"
	. "gopkg.in/check.v1"
	"net/http"
	"strings"
	"testing"
	"time"
)

var HttpStatusOK = int32(http.StatusOK)
var HttpStatusForbidden = int32(http.StatusForbidden)
var HttpStatusNotFound = int32(http.StatusNotFound)
var HttpStatusPreconditionRequired = int32(http.StatusPreconditionRequired)

func Test(t *testing.T) { TestingT(t) }

type UserTestSuite struct {
	server *server.Server
}

var _ = Suite(&UserTestSuite{})

func (ut *UserTestSuite) SetUpSuite(c *C) {
	var err error
	flag.Parse()

	cfg := &server.Config{
		EmailConfirmationTTL: time.Second * 5,
		SMSConfirmationTTL:   time.Second * 2,
	}
	ut.server, err = server.NewServer(cfg)

	c.Assert(err, IsNil)

	go ut.server.RunServer()
	time.Sleep(time.Second * 4)
}

func getDefaultUserEmail() string {
	return "user_admin@simplendi.com"
}

func getDefaultUserPassword() string {
	return "[frth[fr"
}

func getTestLoginToken(cred string) string {
	creds := make(map[string]interface{})
	json.Unmarshal([]byte(cred), &creds)

	body := strings.NewReader(cred)
	resp, err := http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	if err != nil {
		glog.Error(err)
	}

	smsGate := server.GetSMSGateway()
	mess := smsGate.GetLatestMessage()

	messParts := strings.Split(mess.Text, ":")

	codeReq := fmt.Sprintf(`{"sms_code":"%s", "email":"%s", "password":"%s"}`, strings.TrimSpace(messParts[1]), creds["email"].(string), creds["password"].(string))
	body = strings.NewReader(codeReq)

	resp, err = http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	defer resp.Body.Close()

	respObj := server.NewLoginResponse()
	jsonpb.Unmarshal(resp.Body, respObj)
	return "Bearer " + respObj.Token
}

func getTestDefaultAuthToken() string {
	return getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "%s"}`, getDefaultUserEmail(), getDefaultUserPassword()))
}

// test login procedure
func (ut *UserTestSuite) TestLogin(c *C) {
	body := strings.NewReader(fmt.Sprintf(`{"email":"%s", "password": "%s"}`, getDefaultUserEmail(), getDefaultUserPassword()))
	resp, err := http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	loginResp := server.NewLoginResponse()
	err = jsonpb.Unmarshal(resp.Body, loginResp)

	c.Assert(err, IsNil)

	// check if first-phase of login success
	c.Check(loginResp.Meta.StatusCode, Equals, HttpStatusPreconditionRequired)
	c.Assert(loginResp.Meta.Ok, Equals, true)
	c.Assert(loginResp.Meta.Error, Equals, "")
	c.Assert(loginResp.Token, Equals, "")

	// check sms-code
	smsGate := server.GetSMSGateway()
	mess := smsGate.GetLatestMessage()

	c.Assert(mess.To, Equals, "9999")

	messParts := strings.Split(mess.Text, ":")

	// try to complete login process with wrong email
	body = strings.NewReader(fmt.Sprintf(`{"code":"%s", "email":"test@test.com"}`, strings.TrimSpace(messParts[1])))
	resp, err = http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	err = jsonpb.Unmarshal(resp.Body, loginResp)
	c.Assert(err, IsNil)

	c.Assert(loginResp.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(loginResp.Meta.Ok, Equals, false)
	c.Assert(loginResp.Meta.Error, Not(Equals), "")
	c.Assert(loginResp.Token, Equals, "")
}

func (ut *UserTestSuite) TestGetUser(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	testUser, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	user, err := ut.getUser(testUser.Id, token)
	c.Assert(err, IsNil)

	c.Assert(user.Meta.Ok, Equals, true)
	c.Assert(user.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(user.Data.Id, Equals, testUser.Id)
	c.Assert(user.Data.Password, Equals, "")
}

func (ut *UserTestSuite) getUser(id, token string) (*grpc_gateway_user.UserResponse, error) {
	userResp := server.NewUserResponse()

	url := fmt.Sprintf("http://127.0.0.1:8080/v1/user/%v", id)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return userResp, err
	}

	req.Header.Add("Authorization", token)

	cl := server.GetHTTPClient()
	resp, err := cl.Do(req)
	if err != nil {
		return userResp, err
	}

	defer resp.Body.Close()

	err = jsonpb.Unmarshal(resp.Body, userResp)
	return userResp, err
}

func (ut *UserTestSuite) TestGetUserByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	// create user for credentials
	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	_, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	// create user for retreiving information
	email = fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	anotherTestUser, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	userResponse, err := ut.getUser(anotherTestUser.Id, createdUserToken)
	c.Assert(err, IsNil)

	c.Assert(userResponse.Meta.StatusCode, Equals, HttpStatusForbidden)
}

// create user from user with non-admin permissions
func (ut *UserTestSuite) TestCreateByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	_, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	//try to create new user with credentials of non-admin user
	emailForChecking := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	user := grpc_gateway_user.User{
		Name:      "test1",
		Email:     emailForChecking,
		Password:  "12345",
		IsAdmin:   false,
		IsEnabled: true,
	}

	userTxt, _ := json.Marshal(user)
	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v1/user", bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	idResp := server.NewIDResponse()
	err = jsonpb.Unmarshal(resp.Body, idResp)
	c.Assert(err, IsNil)
	c.Assert(idResp.Meta.StatusCode, Equals, HttpStatusForbidden)
}

// create user from user with admin permissions
func (ut *UserTestSuite) TestCreateByAdmin(c *C) {
	token := getTestDefaultAuthToken()

	// try to create new user with credentials of admin user
	emailForChecking := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	user := grpc_gateway_user.User{
		Name:      "test1",
		Email:     emailForChecking,
		Password:  "12345",
		IsAdmin:   false,
		IsEnabled: true,
		Phone:     "+7500",
	}

	userTxt, _ := json.Marshal(user)

	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v1/user", bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	// check server response of creating response
	userResp := server.NewUserResponse()
	err = jsonpb.Unmarshal(resp.Body, userResp)
	c.Assert(err, IsNil)

	c.Assert(userResp.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(userResp.Meta.Ok, Equals, true)
	c.Assert(userResp.Meta.Error, Equals, "")

	// check if email sent
	emailInstance := server.GetEmailSenderInstance()
	sentEmail := emailInstance.GetLatestMessage()
	c.Assert(sentEmail, Not(IsNil))

	emailBody := bytes.NewBufferString("")
	_, err = sentEmail.WriteTo(emailBody)
	c.Assert(err, IsNil)

	// try to login with new user
	body := strings.NewReader(fmt.Sprintf(`{"email":"%s", "password": "%s"}`, user.Email, "12345"))
	resp, err = http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	loginResp := server.NewLoginResponse()
	err = jsonpb.Unmarshal(resp.Body, loginResp)

	c.Assert(err, IsNil)

	// check if first-phase of login not-success
	c.Check(loginResp.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(loginResp.Meta.Ok, Equals, false)
	c.Assert(loginResp.Meta.Error, Not(Equals), "")

	// try to confirm email without sms-code
	emailParts := strings.Split(emailBody.String(), "confirm-email/")
	user.EmailCode = emailParts[1]

	userTxt, _ = json.Marshal(user)
	req, err = http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/confirm-email/%v", user.EmailCode), bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	resp, err = client.Do(req)
	defer resp.Body.Close()

	userResp = server.NewUserResponse()
	err = jsonpb.Unmarshal(resp.Body, userResp)
	c.Assert(err, IsNil)

	c.Assert(userResp.Meta.StatusCode, Equals, HttpStatusPreconditionRequired)
	c.Assert(userResp.Meta.Ok, Equals, true)
	c.Assert(userResp.Meta.Error, Equals, "")

	// try to login with new user
	body = strings.NewReader(fmt.Sprintf(`{"email":"%s", "password": "%s"}`, user.Email, "12345"))
	resp, err = http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	loginResp = server.NewLoginResponse()
	err = jsonpb.Unmarshal(resp.Body, loginResp)

	c.Assert(err, IsNil)

	// check if first-phase of login not-success
	c.Check(loginResp.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(loginResp.Meta.Ok, Equals, false)
	c.Assert(loginResp.Meta.Error, Not(Equals), "")

	// check sms-code
	smsGate := server.GetSMSGateway()
	mess := smsGate.GetLatestMessage()

	c.Assert(mess.To, Equals, "+7500")

	messParts := strings.Split(mess.Text, ":")
	user.SmsCode = strings.TrimSpace(messParts[1])

	// try to confirm email with sms-code
	userTxt, _ = json.Marshal(user)
	req, err = http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/confirm-email/%v", user.EmailCode), bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	resp, err = client.Do(req)
	defer resp.Body.Close()

	userResp = server.NewUserResponse()
	err = jsonpb.Unmarshal(resp.Body, userResp)
	c.Assert(err, IsNil)

	c.Assert(userResp.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(userResp.Meta.Ok, Equals, true)
	c.Assert(userResp.Meta.Error, Equals, "")
	c.Assert(userResp.Data.Id, Not(Equals), "")
	c.Assert(userResp.Data.Password, Equals, "")

	// try to login with new user
	body = strings.NewReader(fmt.Sprintf(`{"email":"%s", "password": "%s"}`, user.Email, "12345"))
	resp, err = http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	loginResp = server.NewLoginResponse()
	err = jsonpb.Unmarshal(resp.Body, loginResp)

	c.Assert(err, IsNil)

	// check if first-phase of login success
	c.Check(loginResp.Meta.StatusCode, Equals, HttpStatusPreconditionRequired)
	c.Assert(loginResp.Meta.Ok, Equals, true)
	c.Assert(loginResp.Meta.Error, Equals, "")
	c.Assert(loginResp.Token, Equals, "")

	// retrieve sms-message
	mess = smsGate.GetLatestMessage()

	// try to retreive user
	userResp, err = ut.getUser(userResp.Data.Id, token)
	c.Assert(err, IsNil)

	c.Assert(userResp.Meta.Ok, Equals, true)
	c.Assert(userResp.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(userResp.Data.Id, Equals, userResp.Data.Id)
}

// create user with email already created in system
func (ut *UserTestSuite) TestCreateExistedUser(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	_, err := createTestUser(email, token, "", true)
	c.Assert(err, IsNil)

	//createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	//try to create new user by admin user credentials
	user := grpc_gateway_user.User{
		Name:      "test1",
		Email:     email,
		Password:  "12345",
		IsAdmin:   false,
		IsEnabled: true,
	}

	userTxt, _ := json.Marshal(user)
	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v1/user", bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	mess := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, mess)
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(mess.Meta.Ok, Equals, false)
	c.Assert(mess.Meta.Error, Not(Equals), "")
}

// delete user
func (ut *UserTestSuite) TestDeleteUser(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	user, err := createTestUser(email, token, "", true)
	c.Assert(err, IsNil)

	// try to delete user
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://127.0.0.1:8080/v1/user/%v", user.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	mess := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, mess)
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.Ok, Equals, true)
	c.Assert(mess.Meta.Error, Equals, "")

	userResponse, err := ut.getUser(user.Id, token)
	c.Assert(err, IsNil)

	c.Assert(userResponse.Meta.StatusCode, Equals, HttpStatusNotFound)

	// try to retrieve user list
	url := "http://127.0.0.1:8080/v1/user"
	req, err = http.NewRequest("GET", url, nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	cl := server.GetHTTPClient()

	resp, err = cl.Do(req)
	c.Assert(err, IsNil)

	users := server.NewUserListResponse()
	err = jsonpb.Unmarshal(resp.Body, users)
	c.Assert(err, IsNil)

	isFound := false
	for _, us := range users.Data {
		if us.Id == user.Id {
			isFound = true
		}

		c.Assert(us.Password, Equals, "")
	}

	c.Assert(isFound, Equals, false)
}

func (ut *UserTestSuite) TestDeleteByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	user, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://127.0.0.1:8080/v1/user/%v", user.Id), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	servResp := server.NewCommonResponse()
	err = jsonpb.Unmarshal(resp.Body, servResp)
	c.Assert(err, IsNil)

	c.Assert(servResp.Meta.StatusCode, Equals, HttpStatusForbidden)
	c.Assert(servResp.Meta.Ok, Equals, false)
	c.Assert(servResp.Meta.Error, Not(Equals), "")
}

func (ut *UserTestSuite) TestGetUsers(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	testUser, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	url := "http://127.0.0.1:8080/v1/user"

	req, err := http.NewRequest("GET", url, nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	cl := server.GetHTTPClient()
	resp, err := cl.Do(req)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	users := server.NewUserListResponse()
	err = jsonpb.Unmarshal(resp.Body, users)
	c.Assert(err, IsNil)

	c.Assert(users.Meta.StatusCode, Equals, HttpStatusOK)

	isFound := false
	for _, us := range users.Data {
		if us.Id == testUser.Id {
			isFound = true
			break
		}
	}

	c.Assert(isFound, Equals, true)
}

func (ut *UserTestSuite) TestGetUsersForNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	_, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	url := "http://127.0.0.1:8080/v1/user"

	req, err := http.NewRequest("GET", url, nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	cl := server.GetHTTPClient()
	resp, err := cl.Do(req)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	users := server.NewUserListResponse()
	err = jsonpb.Unmarshal(resp.Body, users)
	c.Assert(err, IsNil)

	c.Assert(users.Meta.StatusCode, Equals, HttpStatusForbidden)
	c.Assert(len(users.Data), Equals, 0)
}

func (ut *UserTestSuite) TestGetUsersByNonAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	_, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	url := "http://127.0.0.1:8080/v1/user"

	req, err := http.NewRequest("GET", url, nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	cl := server.GetHTTPClient()
	resp, err := cl.Do(req)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	userResponse := server.NewUserResponse()
	err = jsonpb.Unmarshal(resp.Body, userResponse)
	c.Assert(err, IsNil)

	c.Assert(userResponse.Meta.StatusCode, Equals, HttpStatusForbidden)
}

func (ut *UserTestSuite) TestUpdate(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	oldUser, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	// prepare fields for updating
	user := &grpc_gateway_user.User{
		Id:        oldUser.Id,
		Name:      "test234",
		Email:     email + "sdf",
		Password:  "",
		IsAdmin:   true,
		IsEnabled: true,
		Phone:     oldUser.Phone,
	}

	// try to update user
	mess, err := updateTestUser(user.Id, user, createdUserToken, "")
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(mess.Meta.Ok, Equals, true)
	c.Assert(mess.Meta.Error, Equals, "")
	c.Assert(mess.Data.Id, Not(Equals), "")
	c.Assert(mess.Data.Password, Equals, "")

	updatedUser, err := ut.getUser(user.Id, createdUserToken)
	c.Assert(err, IsNil)

	c.Assert(updatedUser.Meta.StatusCode, Equals, HttpStatusOK)

	c.Assert(updatedUser.Data.Email, Equals, user.Email)
	c.Assert(oldUser.Password, Equals, updatedUser.Data.Password)
	c.Assert(updatedUser.Data.Name, Equals, user.Name)

	c.Assert(updatedUser.Data.IsEnabled, Equals, oldUser.IsEnabled)
	c.Assert(updatedUser.Data.IsAdmin, Equals, oldUser.IsAdmin)

	codeReq := fmt.Sprintf(`{"email":"%s", "password":"%s"}`, user.Email, "12345")
	body := strings.NewReader(codeReq)

	resp, err := http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	defer resp.Body.Close()

	respObj := server.NewLoginResponse()
	jsonpb.Unmarshal(resp.Body, respObj)

	c.Assert(respObj.Meta.Ok, Equals, true)
	c.Assert(respObj.Meta.Error, Equals, "")

	smsGate := server.GetSMSGateway()
	smsGate.GetLatestMessage()
}

func (ut *UserTestSuite) TestUpdateByAdmin(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	oldUser, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	// prepare fields for updating
	user := &grpc_gateway_user.User{
		Id:        oldUser.Id,
		Name:      "test234",
		Email:     email + "sdf",
		Password:  "",
		IsAdmin:   true,
		IsEnabled: true,
		Phone:     oldUser.Phone,
	}

	// try to update user
	mess, err := updateTestUser(user.Id, user, token, "")
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(mess.Meta.Ok, Equals, true)
	c.Assert(mess.Meta.Error, Equals, "")
	c.Assert(mess.Data.Id, Not(Equals), "")
	c.Assert(mess.Data.Password, Equals, "")

	updatedUser, err := ut.getUser(user.Id, token)
	c.Assert(err, IsNil)

	c.Assert(updatedUser.Meta.StatusCode, Equals, HttpStatusOK)

	c.Assert(updatedUser.Data.Email, Equals, user.Email)
	c.Assert(oldUser.Password, Equals, updatedUser.Data.Password)
	c.Assert(updatedUser.Data.Name, Equals, user.Name)

	c.Assert(updatedUser.Data.IsEnabled, Equals, oldUser.IsEnabled)
	c.Assert(updatedUser.Data.IsAdmin, Equals, true)

	codeReq := fmt.Sprintf(`{"email":"%s", "password":"%s"}`, user.Email, "12345")
	body := strings.NewReader(codeReq)

	resp, err := http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	defer resp.Body.Close()

	respObj := server.NewLoginResponse()
	jsonpb.Unmarshal(resp.Body, respObj)

	c.Assert(respObj.Meta.Ok, Equals, true)
	c.Assert(respObj.Meta.Error, Equals, "")

	smsGate := server.GetSMSGateway()
	smsGate.GetLatestMessage()
}

func (ut *UserTestSuite) TestUpdatePhoneNumber(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	oldUser, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	// prepare fields for updating
	user := &grpc_gateway_user.User{
		Id:       oldUser.Id,
		Phone:    "8888",
		Email:    oldUser.Email,
		Name:     oldUser.Name,
		Password: oldUser.Password,
	}

	// try to update user
	mess, err := updateTestUser(user.Id, user, createdUserToken, "")
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusPreconditionRequired)
	c.Assert(mess.Meta.Ok, Equals, true)
	c.Assert(mess.Meta.Error, Equals, "")

	// check sms-code
	smsGate := server.GetSMSGateway()
	smsMessage := smsGate.GetLatestMessage()

	c.Assert(smsMessage, NotNil)
	c.Assert(smsMessage.To, Equals, "9999")

	// try to update user with wrong code
	user.SmsCode = "343"
	mess, err = updateTestUser(user.Id, user, createdUserToken, "343")
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusForbidden)
	c.Assert(mess.Meta.Ok, Equals, false)
	c.Assert(mess.Meta.Error, Equals, server.ErrSMSConfirmationFailed.Error())

	// try to update user with correct code
	messParts := strings.Split(smsMessage.Text, ":")
	user.SmsCode = strings.TrimSpace(messParts[1])

	mess, err = updateTestUser(user.Id, user, createdUserToken, strings.TrimSpace(messParts[1]))
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(mess.Meta.Ok, Equals, true)
	c.Assert(mess.Meta.Error, Equals, "")

	updatedUser, err := ut.getUser(user.Id, createdUserToken)
	c.Assert(err, IsNil)

	c.Assert(updatedUser.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(updatedUser.Data.Phone, Equals, user.Phone)
}

func (ut *UserTestSuite) TestUpdateNotOwner(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	oldUser, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	accessEmail := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	_, err = createTestUser(accessEmail, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, accessEmail))

	// prepare fields for updating
	user := &grpc_gateway_user.User{
		Id:        oldUser.Id,
		Name:      "test234",
		Email:     email + "sdf",
		Password:  "1234567",
		IsAdmin:   true,
		IsEnabled: true,
		Phone:     oldUser.Phone,
	}

	// try to update user
	userTxt, _ := json.Marshal(user)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/user/%v", user.Id), bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	mess := server.NewUserResponse()
	err = jsonpb.Unmarshal(resp.Body, mess)
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusForbidden)
	c.Assert(mess.Meta.Ok, Equals, false)
	c.Assert(mess.Meta.Error, Not(Equals), "")
}

func (ut *UserTestSuite) TestGetUsersByCompany(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	user1, err := createTestUser(email, token, "1234", false)
	c.Assert(err, IsNil)

	email = fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	user2, err := createTestUser(email, token, "1234", false)
	c.Assert(err, IsNil)

	email = fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())
	user3, err := createTestUser(email, token, "2345", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, user1.Email))

	// try to get user list
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/user_by_company/%v", user1.CompanyId), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	users := server.NewUserListResponse()
	err = jsonpb.Unmarshal(resp.Body, users)
	c.Assert(err, IsNil)

	c.Assert(users.Meta.StatusCode, Equals, HttpStatusOK)

	foundWithOtherCompanyId := 0
	foundCount := 0
	for _, us := range users.Data {
		if us.Id == user1.Id || us.Id == user2.Id {
			foundCount += 1
		}

		if us.CompanyId == "2345" {
			foundWithOtherCompanyId += 1
		}
	}

	c.Assert(foundWithOtherCompanyId, Equals, 0)
	c.Assert(foundCount, Equals, 2)

	// try to get user list from other company'ut user
	createdUserToken = getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, user3.Email))
	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/user_by_company/%v", user1.CompanyId), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", createdUserToken)

	resp, err = client.Do(req)
	defer resp.Body.Close()

	userListResponse := server.NewUserListResponse()
	err = jsonpb.Unmarshal(resp.Body, userListResponse)
	c.Assert(err, IsNil)

	c.Assert(userListResponse.Meta.StatusCode, Equals, HttpStatusForbidden)

	// try to get user list from admin user
	req, err = http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8080/v1/user_by_company/%v", user1.CompanyId), nil)
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	resp, err = client.Do(req)
	defer resp.Body.Close()

	userListResponse = server.NewUserListResponse()
	err = jsonpb.Unmarshal(resp.Body, userListResponse)
	c.Assert(err, IsNil)

	c.Assert(userListResponse.Meta.StatusCode, Equals, HttpStatusOK)
}

func (ut *UserTestSuite) TestEmailConfirmationEmailTokenTimeToLive(c *C) {
	token := getTestDefaultAuthToken()
	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	user := &grpc_gateway_user.User{
		Id:        "",
		Name:      "test234",
		Email:     email,
		IsAdmin:   true,
		IsEnabled: true,
	}

	userResp, err := rawCreateTestUser(user, token)
	c.Assert(err, IsNil)

	// check if email sent
	emailInstance := server.GetEmailSenderInstance()
	sentEmail := emailInstance.GetLatestMessage()
	c.Assert(sentEmail, Not(IsNil))

	emailBody := bytes.NewBufferString("")
	_, err = sentEmail.WriteTo(emailBody)
	c.Assert(err, IsNil)

	parts := strings.Split(emailBody.String(), "/confirm-email/")

	time.Sleep(ut.server.Config.EmailConfirmationTTL * 2)

	userTxt, _ := json.Marshal(user)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/confirm-email/%v", parts[1]), bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	cl := server.GetHTTPClient()

	resp, err := cl.Do(req)
	defer resp.Body.Close()

	err = jsonpb.Unmarshal(resp.Body, userResp)
	c.Assert(err, IsNil)

	smsGate := server.GetSMSGateway()
	smsGate.GetLatestMessage()

	c.Assert(userResp.Meta.Ok, Equals, false)
	c.Assert(userResp.Meta.Error, Equals, server.ErrEmailCodeExpired.Error())
}

func (ut *UserTestSuite) TestEmailConfirmationSMSTokenTimeToLive(c *C) {
	token := getTestDefaultAuthToken()
	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	user := &grpc_gateway_user.User{
		Id:        "",
		Name:      "test234",
		Email:     email,
		IsAdmin:   true,
		IsEnabled: true,
	}

	userResp, err := rawCreateTestUser(user, token)
	c.Assert(err, IsNil)

	// check if email sent
	emailInstance := server.GetEmailSenderInstance()
	sentEmail := emailInstance.GetLatestMessage()
	c.Assert(sentEmail, Not(IsNil))

	emailBody := bytes.NewBufferString("")
	_, err = sentEmail.WriteTo(emailBody)
	c.Assert(err, IsNil)

	parts := strings.Split(emailBody.String(), "/confirm-email/")
	user.EmailCode = parts[1]

	userTxt, _ := json.Marshal(user)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/confirm-email/%v", user.EmailCode), bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	cl := server.GetHTTPClient()

	resp, err := cl.Do(req)
	defer resp.Body.Close()

	err = jsonpb.Unmarshal(resp.Body, userResp)
	c.Assert(err, IsNil)

	smsGate := server.GetSMSGateway()
	mess := smsGate.GetLatestMessage()

	messParts := strings.Split(mess.Text, ":")
	user.SmsCode = strings.TrimSpace(messParts[1])

	time.Sleep(ut.server.Config.SMSConfirmationTTL * 2)

	// try to confirm email with sms-code
	userTxt, _ = json.Marshal(user)
	req, err = http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/confirm-email/%v", user.EmailCode), bytes.NewReader(userTxt))
	c.Assert(err, IsNil)

	req.Header.Add("Authorization", token)

	resp, err = cl.Do(req)
	defer resp.Body.Close()

	userResp = server.NewUserResponse()
	err = jsonpb.Unmarshal(resp.Body, userResp)
	c.Assert(err, IsNil)

	c.Assert(userResp.Meta.Ok, Equals, false)
	c.Assert(userResp.Meta.Error, Equals, server.ErrSMSCodeExpired.Error())
}

func (ut *UserTestSuite) TestLoginSMSTTL(c *C) {
	body := strings.NewReader(fmt.Sprintf(`{"email":"%s", "password": "%s"}`, getDefaultUserEmail(), getDefaultUserPassword()))
	resp, err := http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	loginResp := server.NewLoginResponse()
	err = jsonpb.Unmarshal(resp.Body, loginResp)

	c.Assert(err, IsNil)

	// check if first-phase of login success
	c.Check(loginResp.Meta.StatusCode, Equals, HttpStatusPreconditionRequired)
	c.Assert(loginResp.Meta.Ok, Equals, true)
	c.Assert(loginResp.Meta.Error, Equals, "")
	c.Assert(loginResp.Token, Equals, "")

	time.Sleep(ut.server.Config.SMSConfirmationTTL * 2)

	// check sms-code
	smsGate := server.GetSMSGateway()
	mess := smsGate.GetLatestMessage()

	c.Assert(mess.To, Equals, "9999")

	messParts := strings.Split(mess.Text, ":")

	// try to complete login process with correct email and sms_code
	codeReq := fmt.Sprintf(`{"sms_code":"%s", "email":"%s", "password":"%s"}`, strings.TrimSpace(messParts[1]), getDefaultUserEmail(), getDefaultUserPassword())
	body = strings.NewReader(codeReq)

	resp, err = http.Post("http://127.0.0.1:8080/v1/login", "application/json", body)
	c.Assert(err, IsNil)

	defer resp.Body.Close()

	err = jsonpb.Unmarshal(resp.Body, loginResp)
	c.Assert(err, IsNil)

	c.Assert(loginResp.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(loginResp.Meta.Ok, Equals, false)
	c.Assert(loginResp.Meta.Error, Equals, server.ErrSMSCodeExpired.Error())
}

func (ut *UserTestSuite) TestUpdatePhoneNumberSMSTTL(c *C) {
	token := getTestDefaultAuthToken()

	email := fmt.Sprintf("test_%v@test.com", time.Now().UnixNano())

	oldUser, err := createTestUser(email, token, "", false)
	c.Assert(err, IsNil)

	createdUserToken := getTestLoginToken(fmt.Sprintf(`{"email":"%s", "password": "12345"}`, email))

	// prepare fields for updating
	user := &grpc_gateway_user.User{
		Id:       oldUser.Id,
		Phone:    "8888",
		Email:    oldUser.Email,
		Name:     oldUser.Name,
		Password: oldUser.Password,
	}

	// try to update user
	mess, err := updateTestUser(user.Id, user, createdUserToken, "")
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusPreconditionRequired)
	c.Assert(mess.Meta.Ok, Equals, true)
	c.Assert(mess.Meta.Error, Equals, "")

	// check sms-code
	smsGate := server.GetSMSGateway()
	smsMessage := smsGate.GetLatestMessage()

	c.Assert(smsMessage, NotNil)
	c.Assert(smsMessage.To, Equals, "9999")

	// try to update user with correct code
	messParts := strings.Split(smsMessage.Text, ":")
	user.SmsCode = strings.TrimSpace(messParts[1])

	time.Sleep(ut.server.Config.SMSConfirmationTTL * 2)
	mess, err = updateTestUser(user.Id, user, createdUserToken, strings.TrimSpace(messParts[1]))
	c.Assert(err, IsNil)

	c.Assert(mess.Meta.StatusCode, Equals, HttpStatusOK)
	c.Assert(mess.Meta.Ok, Equals, false)
	c.Assert(mess.Meta.Error, Equals, server.ErrSMSCodeExpired.Error())
}
