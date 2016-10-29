package server

import (
	"errors"
	grpc_gateway_user "git.simplendi.com/FirmQ/frontend-server/server/proto/user"
	"github.com/satori/go.uuid"
	//"github.com/tvdburgt/go-argon2"
	"github.com/golang/protobuf/ptypes/timestamp"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"time"
)

// ErrSMSConfirmationRequired - error which server returns when action required sms confirmation
var ErrSMSConfirmationRequired = errors.New("you should confirm request with code from sms")

// ErrSMSConfirmationFailed - error which server returns when verification procedure of sms confirmation failed
var ErrSMSConfirmationFailed = errors.New("you passed wrong code from sms")

// ErrOwner - error when server require owner for some action
var ErrOwner = errors.New("you should be owner of this object for making any changes")

// ErrEmailCodeExpired - error when email code is expired, needs to be regenerated
var ErrEmailCodeExpired = errors.New("email code for this user is expired")

// ErrSMSCodeExpired - error when sms-code is expired, needs to be regenerated
var ErrSMSCodeExpired = errors.New("sms-code for this user is expired")

// UserRepo - model for accessing users in database
type UserRepo struct {
	sess *mgo.Database
	coll string
}

// NewUserRepo - returns new instance of UserRepo which provide access to user model
func NewUserRepo(sess *mgo.Database) *UserRepo {
	return &UserRepo{
		sess: sess,
		coll: "users",
	}
}

// CreateUser - create new user
func (ur *UserRepo) CreateUser(user *grpc_gateway_user.User) error {
	c := ur.sess.C(ur.coll)

	user.Id = uuid.NewV4().String()

	// don't set password and phone until email isn't confirmed
	user.Password = ""
	user.Phone = ""
	user.IsEnabled = true

	return c.Insert(user)
}

// GetUserByID - get user from database by id
func (ur *UserRepo) GetUserByID(id string) (*grpc_gateway_user.User, error) {
	c := ur.sess.C(ur.coll)
	var user grpc_gateway_user.User

	err := c.Find(bson.M{"id": id, "isenabled": true, "isconfirmed": true}).One(&user)
	return &user, err
}

// GetUserByEmailCode - get user from database by id
func (ur *UserRepo) GetUserByEmailCode(user *grpc_gateway_user.User) (*grpc_gateway_user.User, error) {
	c := ur.sess.C(ur.coll)

	storedUser := new(grpc_gateway_user.User)
	err := c.Find(bson.M{"emailcode": user.EmailCode}).One(&storedUser)
	return storedUser, err
}

// SetSMSCode - set sms-code for specific user
func (ur *UserRepo) SetSMSCode(userID, code string) error {
	c := ur.sess.C(ur.coll)

	tm := timestamp.Timestamp{Seconds: time.Now().Unix()}
	return c.Update(bson.M{"id": userID}, bson.M{"$set": bson.M{"smscode": code, "smssentat": tm}})
}

// EnableUserAndSetPasswordPhone - enable user after success confirmation
func (ur *UserRepo) EnableUserAndSetPasswordPhone(userID, password, phone string) error {
	c := ur.sess.C(ur.coll)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return c.Update(bson.M{"id": userID}, bson.M{"$set": bson.M{
		"emailcode":   "",
		"smscode":     "",
		"emailsentat": nil,
		"smssentat":   nil,
		"isconfirmed": true,
		"password":    string(hash),
		"phone":       phone,
	}})
}

// LoginUser - check if user passed correct credentials
func (ur *UserRepo) LoginUser(email, password string) (*grpc_gateway_user.User, error) {
	c := ur.sess.C(ur.coll)
	var user grpc_gateway_user.User

	err := c.Find(bson.M{"email": email, "isenabled": true, "isconfirmed": true}).One(&user)
	if err != nil {
		return &user, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return &user, err
	}

	return &user, err
}

// GetUserBySMSCode - find user by email and sms code. Uses in all confirmation-required services
func (ur *UserRepo) GetUserBySMSCode(email, code string) (*grpc_gateway_user.User, error) {
	c := ur.sess.C(ur.coll)
	var user grpc_gateway_user.User

	err := c.Find(bson.M{"email": email, "smscode": code}).One(&user)
	return &user, err
}

// ConfirmSMSUser - confirm sms-code passed from user
func (ur *UserRepo) ConfirmSMSUser(email, code string) error {
	c := ur.sess.C(ur.coll)
	err := c.Update(bson.M{"email": email, "smscode": code}, bson.M{"$set": bson.M{"smscode": "", "smssentat": nil}})

	return err
}

// DeleteUserByID - set user as disabled from database by id
func (ur *UserRepo) DeleteUserByID(id string) error {
	c := ur.sess.C(ur.coll)
	err := c.Update(bson.M{"id": id}, bson.M{"$set": bson.M{"isenabled": false}})
	return err
}

// GetUsers - get users from database
func (ur *UserRepo) GetUsers() (*grpc_gateway_user.UserListResponse, error) {
	c := ur.sess.C(ur.coll)
	users := NewUserListResponse()

	err := c.Find(bson.M{"isenabled": true, "isconfirmed": true}).All(&users.Data)
	return users, err
}

// GetUsersByCompanyID - get users from database by company id
func (ur *UserRepo) GetUsersByCompanyID(companyID string) (*grpc_gateway_user.UserListResponse, error) {
	c := ur.sess.C(ur.coll)
	users := NewUserListResponse()

	err := c.Find(bson.M{"isenabled": true, "companyid": companyID}).All(&users.Data)
	return users, err
}

// UpdateUserByID - update user information by user id
func (ur *UserRepo) UpdateUserByID(isAdminUser bool, oldUser, user *grpc_gateway_user.User) (*grpc_gateway_user.User, error) {
	c := ur.sess.C(ur.coll)

	if user.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		//hash, err := argon2.Hash(argon2.NewContext(), []byte(user.Password), []byte("somesalt"))
		if err != nil {
			return oldUser, err
		}

		oldUser.Password = string(hash)
	}

	// if user admin - allow set admin user
	if isAdminUser {
		oldUser.IsAdmin = user.IsAdmin
		oldUser.CompanyId = user.CompanyId
	}

	// update allowed fields
	oldUser.Email = user.Email
	oldUser.Name = user.Name
	oldUser.Phone = user.Phone
	oldUser.SmsCode = ""
	oldUser.SmsSentAt = nil

	err := c.Update(bson.M{"id": user.Id}, oldUser)
	return oldUser, err
}

// CreateIndexes - create necessary indexes for fast executing
func (ur *UserRepo) CreateIndexes() {
	c := ur.sess.C(ur.coll)
	c.EnsureIndex(mgo.Index{
		Key:    []string{"email"},
		Unique: true,
	})

	c.EnsureIndex(mgo.Index{
		Key:    []string{"id"},
		Unique: true,
	})
}
