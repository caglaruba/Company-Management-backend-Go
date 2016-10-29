package server

import (
	"errors"
	"fmt"
	grpc_gateway_common "git.simplendi.com/FirmQ/frontend-server/server/proto/common"
	grpc_gateway_user "git.simplendi.com/FirmQ/frontend-server/server/proto/user"
	"github.com/dgrijalva/jwt-go"
	google_protobuf1 "github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"net/http"
	"strings"
	"time"
)

type userServer struct {
	config *Config
}

// GetCurrentUser - retrieve current from context
func GetCurrentUser(ctx context.Context) (*grpc_gateway_user.User, error) {
	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		return nil, err
	}

	userRepo := NewUserRepo(sess)

	userID := ctx.Value("user_id").(string)
	return userRepo.GetUserByID(userID)
}

// IsAdminUser - check if current user has admin permissions
func IsAdminUser(ctx context.Context) error {
	currentUser, err := GetCurrentUser(ctx)
	if err != nil {
		err = fmt.Errorf("you should have admin permission for this action")
		return err
	}

	if currentUser.IsAdmin == false {
		err = fmt.Errorf("you should have admin permission for this action")
		return err
	}

	return nil
}

// IsAdminOrCompanyUser - check if current user has admin permissions or linked with companyID company
func IsAdminOrCompanyUser(ctx context.Context, companyID string) error {
	currentUser, err := GetCurrentUser(ctx)
	if err != nil {
		err = fmt.Errorf("you should have admin permission or be company user for this action")
		return err
	}

	if currentUser.IsAdmin == false && currentUser.CompanyId != companyID {
		err = fmt.Errorf("you should have admin permission or be company user for this action")
		return err
	}

	return nil
}

// NewCommonResponse - create  new instance of common response
func NewCommonResponse() *grpc_gateway_common.CommonResponse {
	message := &grpc_gateway_common.CommonResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	return message
}

// NewIDResponse - create new instance of reposponse with id
func NewIDResponse() *grpc_gateway_common.IDResponse {
	message := &grpc_gateway_common.IDResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	return message
}

// NewLoginResponse - create new instance of login response
func NewLoginResponse() *grpc_gateway_user.LoginResponse {
	message := &grpc_gateway_user.LoginResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	return message
}

// NewUserResponse - create new instance of user response
func NewUserResponse() *grpc_gateway_user.UserResponse {
	message := &grpc_gateway_user.UserResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	return message
}

// NewUserListResponse - create new instance of user list response
func NewUserListResponse() *grpc_gateway_user.UserListResponse {
	message := &grpc_gateway_user.UserListResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	return message
}

func filterUserResponseFields(resp *grpc_gateway_user.UserResponse) {
	if resp.Data != nil {
		resp.Data.Password = ""
		resp.Data.SmsCode = ""
	}
}

func filterUserListResponseFields(resp *grpc_gateway_user.UserListResponse) {
	for _, us := range resp.Data {
		us.Password = ""
	}
}

// NewUserServer - returns new grpc server which provide user-related functionality
func NewUserServer(config *Config) grpc_gateway_user.UserServiceServer {
	us := &userServer{
		config: config,
	}

	return us
}

func (s *userServer) Login(ctx context.Context, msg *grpc_gateway_user.LoginRequest) (message *grpc_gateway_user.LoginResponse, err error) {
	message = NewLoginResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	smsGw := GetSMSGateway()
	code := smsGw.GenerateRandomCode(6)
	repo := NewUserRepo(sess)

	// if 1st step of authorization
	if msg.SmsCode == "" {
		user, err := repo.LoginUser(msg.Email, msg.Password)
		if err != nil {
			message.Meta.Ok = false
			message.Meta.Error = err.Error()
			return message, nil
		}

		repo.SetSMSCode(user.Id, code)
		smsGw.SendSMSMessage(user.Phone, code)

		message.Meta.Ok = true
		message.Meta.StatusCode = http.StatusPreconditionRequired

		return message, nil
	}

	storedUser, err := repo.GetUserBySMSCode(msg.Email, msg.SmsCode)
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = ErrSMSConfirmationFailed.Error()
		return message, nil
	}

	if time.Unix(storedUser.SmsSentAt.Seconds, 0).Add(s.config.SMSConfirmationTTL).Before(time.Now()) {
		message.Meta.Ok = false
		message.Meta.Error = ErrSMSCodeExpired.Error()
		return message, nil
	}

	err = repo.ConfirmSMSUser(msg.Email, msg.SmsCode)
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = ErrSMSConfirmationFailed.Error()
		return message, nil
	}

	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["admin"] = storedUser.IsAdmin
	claims["name"] = storedUser.Name
	claims["company_id"] = storedUser.CompanyId
	claims["user_id"] = storedUser.Id
	claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

	//Sign the token with our secret
	tokenString, _ := token.SignedString(secretKey)

	message.Meta.Ok = true
	message.Token = tokenString

	return message, nil
}

func (s *userServer) CreateUser(ctx context.Context, user *grpc_gateway_user.User) (*grpc_gateway_user.UserResponse, error) {
	message := NewUserResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	repo := NewUserRepo(sess)

	if err = IsAdminUser(ctx); err != nil {
		message.Meta.StatusCode = http.StatusForbidden
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	code := uuid.NewV4().String()
	user.EmailCode = code
	user.EmailSentAt = &timestamp.Timestamp{}
	user.EmailSentAt.Seconds = time.Now().Unix()

	err = repo.CreateUser(user)

	if err == nil {
		emailInstance := GetEmailSenderInstance()
		emailInstance.SendEmailConfirmation(user.Name, user.Email, user.EmailCode)
		message.Meta.Ok = true
	} else {
		message.Meta.Ok = false

		if strings.Index(err.Error(), "duplicate") != -1 {
			err = errors.New("user with this email already exist")
			message.Meta.Error = err.Error()
		} else {
			message.Meta.Error = err.Error()
		}

	}

	return message, nil
}

func (s *userServer) ConfirmEmail(ctx context.Context, incomeUser *grpc_gateway_user.User) (*grpc_gateway_user.UserResponse, error) {
	message := NewUserResponse()
	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	repo := NewUserRepo(sess)

	storedUser, err := repo.GetUserByEmailCode(incomeUser)
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	if time.Unix(storedUser.EmailSentAt.Seconds, 0).Add(s.config.EmailConfirmationTTL).Before(time.Now()) {
		message.Meta.Ok = false
		message.Meta.Error = ErrEmailCodeExpired.Error()
		return message, nil
	}

	// if user passed sms-confirmation
	if incomeUser.SmsCode != "" {
		if time.Unix(storedUser.SmsSentAt.Seconds, 0).Add(s.config.SMSConfirmationTTL).Before(time.Now()) {
			message.Meta.Ok = false
			message.Meta.Error = ErrSMSCodeExpired.Error()
			return message, nil
		}

		if storedUser.SmsCode == incomeUser.SmsCode {
			if err := repo.EnableUserAndSetPasswordPhone(storedUser.Id, incomeUser.Password, incomeUser.Phone); err != nil {
				message.Meta.Ok = false
				message.Meta.Error = err.Error()
				return message, nil
			}

		} else {
			message.Meta.Ok = false
			message.Meta.Error = ErrSMSConfirmationFailed.Error()
			return message, nil
		}
	} else {
		smsGw := GetSMSGateway()
		code := smsGw.GenerateRandomCode(6)

		// set sms code
		if err := repo.SetSMSCode(storedUser.Id, code); err != nil {
			message.Meta.Ok = false
			message.Meta.Error = err.Error()
			return message, nil
		}

		if err := smsGw.SendSMSMessage(incomeUser.Phone, code); err != nil {
			message.Meta.Ok = false
			message.Meta.Error = err.Error()
			return message, nil
		}

		message.Meta.Ok = true
		message.Meta.StatusCode = http.StatusPreconditionRequired
		return message, nil
	}

	message.Meta.Ok = true
	message.Data, err = repo.GetUserByID(storedUser.Id)
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	filterUserResponseFields(message)
	return message, nil
}

func (s *userServer) GetUser(ctx context.Context, id *grpc_gateway_common.IDRequest) (user *grpc_gateway_user.UserResponse, err error) {
	user = NewUserResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		user.Meta.Ok = false
		user.Meta.Error = err.Error()
		return user, nil
	}

	repo := NewUserRepo(sess)

	userID := ctx.Value("user_id").(string)

	if err = IsAdminUser(ctx); userID != id.Id && err != nil {
		user.Meta.StatusCode = http.StatusForbidden
		user.Meta.Ok = false
		user.Meta.Error = err.Error()
		return user, nil
	}

	user.Data, err = repo.GetUserByID(id.Id)

	if err == mgo.ErrNotFound {
		user.Meta.StatusCode = http.StatusNotFound
		user.Meta.Ok = false
		user.Meta.Error = err.Error()
	} else {
		if err != nil {
			user.Meta.Ok = false
			user.Meta.Error = err.Error()
		} else {
			user.Meta.Ok = true
		}
	}

	filterUserResponseFields(user)
	return user, nil
}

func (s *userServer) DeleteUser(ctx context.Context, in *grpc_gateway_common.IDRequest) (message *grpc_gateway_common.CommonResponse, err error) {
	message = NewCommonResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	userRepo := NewUserRepo(sess)

	if err = IsAdminUser(ctx); err != nil {
		message.Meta.StatusCode = http.StatusForbidden
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	err = userRepo.DeleteUserByID(in.Id)
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
	} else {
		message.Meta.Ok = true
	}

	return message, nil
}

func (s *userServer) GetUserByCompany(ctx context.Context, in *grpc_gateway_common.IDRequest) (users *grpc_gateway_user.UserListResponse, err error) {
	users = NewUserListResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		users.Meta.Ok = false
		users.Meta.Error = err.Error()
		return users, nil
	}

	if err = IsAdminOrCompanyUser(ctx, in.Id); err != nil {
		users.Meta.StatusCode = http.StatusForbidden
		users.Meta.Ok = false
		users.Meta.Error = err.Error()
		return users, nil

	}

	userRepo := NewUserRepo(sess)

	users, err = userRepo.GetUsersByCompanyID(in.Id)
	if err != nil {
		users.Meta.Ok = false
		users.Meta.Error = err.Error()
	} else {
		users.Meta.Ok = true
	}

	return users, nil

}

func (s *userServer) GetUsers(ctx context.Context, in *google_protobuf1.Empty) (users *grpc_gateway_user.UserListResponse, err error) {
	users = NewUserListResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		users.Meta.Ok = false
		users.Meta.Error = err.Error()
		return users, nil
	}

	if err = IsAdminUser(ctx); err != nil {
		users.Meta.StatusCode = http.StatusForbidden
		users.Meta.Ok = false
		users.Meta.Error = err.Error()
		return users, nil
	}

	userRepo := NewUserRepo(sess)

	users, err = userRepo.GetUsers()

	filterUserListResponseFields(users)
	if err != nil {
		users.Meta.Ok = false
		users.Meta.Error = err.Error()
	} else {
		users.Meta.Ok = true
	}

	return users, nil
}

func (s *userServer) UpdateUser(ctx context.Context, in *grpc_gateway_user.User) (message *grpc_gateway_user.UserResponse, err error) {
	message = NewUserResponse()

	defer func() {
		filterUserResponseFields(message)
	}()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	repo := NewUserRepo(sess)
	userID := ctx.Value("user_id").(string)

	isAdminUser := IsAdminUser(ctx) == nil
	if in.Id != userID && !isAdminUser {
		message.Meta.StatusCode = http.StatusForbidden
		message.Meta.Ok = false
		message.Meta.Error = ErrOwner.Error()
		return message, nil
	}

	smsGw := GetSMSGateway()
	newSMSCode := smsGw.GenerateRandomCode(6)

	storedUser, err := repo.GetUserByID(in.Id)
	message.Data = storedUser

	if storedUser.Phone != in.Phone {
		if in.SmsCode == "" {
			repo.SetSMSCode(storedUser.Id, newSMSCode)
			smsGw.SendSMSMessage(storedUser.Phone, newSMSCode)
			message.Meta.Ok = true
			message.Meta.Error = ""
			message.Meta.StatusCode = http.StatusPreconditionRequired
			return message, nil
		}

		if time.Unix(storedUser.SmsSentAt.Seconds, 0).Add(s.config.SMSConfirmationTTL).Before(time.Now()) {
			message.Meta.Ok = false
			message.Meta.Error = ErrSMSCodeExpired.Error()
			return message, nil
		}

		if storedUser.SmsCode != in.SmsCode {
			message.Meta.Ok = false
			message.Meta.Error = ErrSMSConfirmationFailed.Error()
			message.Meta.StatusCode = http.StatusForbidden
			return message, nil
		}
	}

	storedUser, err = repo.UpdateUserByID(isAdminUser, storedUser, in)
	message.Data = storedUser

	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
	} else {
		message.Meta.Ok = true
	}
	//case mgo.ErrNotFound:
	//	message.Meta.Ok = false
	//	message.Meta.Error = ErrNotFound.Error()

	return message, nil
}

func (s *userServer) createDefaultUser() error {
	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		return err
	}

	repo := NewUserRepo(sess)
	user := &grpc_gateway_user.User{
		Name:        "Default admin user",
		Email:       "user_admin@simplendi.com",
		Phone:       "9999",
		IsAdmin:     true,
		IsEnabled:   true,
		IsConfirmed: true,
		Password:    "[frth[fr",
	}

	// create required indexes in user collection
	repo.CreateIndexes()

	err = repo.CreateUser(user)
	if err == mgo.ErrNotFound {
		return err
	}

	return nil
}
