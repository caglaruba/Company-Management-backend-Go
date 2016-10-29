package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"git.simplendi.com/FirmQ/frontend-server/server"
	grpc_gateway_user "git.simplendi.com/FirmQ/frontend-server/server/proto/user"
	"github.com/golang/protobuf/jsonpb"
	"net/http"
	"strings"
)

func createTestUser(email, token, companyId string, isAdmin bool) (*grpc_gateway_user.User, error) {
	url := "http://127.0.0.1:8080/v1/user"
	password := "12345"

	user := &grpc_gateway_user.User{
		Name:      "test1",
		Email:     email,
		Password:  password,
		IsEnabled: true,
		CompanyId: companyId,
		IsAdmin:   isAdmin,
		Phone:     "9999",
	}

	userTxt, _ := json.Marshal(user)
	req, err := http.NewRequest("POST", url, bytes.NewReader(userTxt))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	// check if email sent
	emailInstance := server.GetEmailSenderInstance()
	sentEmail := emailInstance.GetLatestMessage()

	emailBody := bytes.NewBufferString("")
	_, err = sentEmail.WriteTo(emailBody)

	// try to confirm email without sms-code
	emailParts := strings.Split(emailBody.String(), "confirm-email/")
	user.EmailCode = emailParts[1]

	userTxt, _ = json.Marshal(user)
	req, err = http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/confirm-email/%v", user.EmailCode), bytes.NewReader(userTxt))

	req.Header.Add("Authorization", token)

	resp, err = client.Do(req)
	defer resp.Body.Close()

	// check sms-code
	smsGate := server.GetSMSGateway()
	mess := smsGate.GetLatestMessage()

	messParts := strings.Split(mess.Text, ":")
	user.SmsCode = strings.TrimSpace(messParts[1])

	// try to confirm email with sms-code
	userTxt, _ = json.Marshal(user)
	req, err = http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/confirm-email/%v", user.EmailCode), bytes.NewReader(userTxt))

	req.Header.Add("Authorization", token)

	resp, err = client.Do(req)
	defer resp.Body.Close()

	userResp := server.NewUserResponse()
	err = jsonpb.Unmarshal(resp.Body, userResp)

	//user.Id = userResp.DataId
	return userResp.Data, nil
}

func updateTestUser(id string, obj interface{}, token, smsCode string) (*grpc_gateway_user.UserResponse, error) {
	mess := server.NewUserResponse()
	body, _ := json.Marshal(obj)

	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8080/v1/user/%v", id), bytes.NewReader(body))
	if err != nil {
		return mess, err
	}

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	err = jsonpb.Unmarshal(resp.Body, mess)
	return mess, err
}

func rawCreateTestUser(user *grpc_gateway_user.User, token string) (*grpc_gateway_user.UserResponse, error) {
	mess := server.NewUserResponse()

	userTxt, err := json.Marshal(user)
	if err != nil {
		return mess, err
	}

	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v1/user", bytes.NewReader(userTxt))
	if err != nil {
		return mess, err
	}

	req.Header.Add("Authorization", token)

	client := server.GetHTTPClient()

	resp, err := client.Do(req)
	defer resp.Body.Close()

	err = jsonpb.Unmarshal(resp.Body, mess)
	return mess, err
}
