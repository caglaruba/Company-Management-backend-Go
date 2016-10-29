package server

import (
	"errors"
	grpc_gateway_company "git.simplendi.com/FirmQ/frontend-server/server/proto/company"
	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// ErrNotFound - error when server cannot find the object
var ErrNotFound = errors.New("cannot find requested object")

// CompanyRepo - model for accessing companys in database
type CompanyRepo struct {
	sess *mgo.Database
	coll string
}

// NewCompanyRepo - returns new instance of CompanyRepo which provide access to company model
func NewCompanyRepo(sess *mgo.Database) *CompanyRepo {
	return &CompanyRepo{
		sess: sess,
		coll: "companies",
	}
}

// CreateCompany - create new company
func (cr *CompanyRepo) CreateCompany(company *grpc_gateway_company.Company) error {
	c := cr.sess.C(cr.coll)

	company.Id = uuid.NewV4().String()
	company.IsEnabled = true
	return c.Insert(company)
}

// GetCompanyByID - get company from database by id
func (cr *CompanyRepo) GetCompanyByID(id string) (*grpc_gateway_company.Company, error) {
	c := cr.sess.C(cr.coll)
	var company grpc_gateway_company.Company

	err := c.Find(bson.M{"id": id, "isenabled": true}).One(&company)
	return &company, err
}

// GetCompanies - get companies from database
func (cr *CompanyRepo) GetCompanies() (*grpc_gateway_company.CompanyListResponse, error) {
	c := cr.sess.C(cr.coll)
	companies := NewCompanyListResponse()

	err := c.Find(bson.M{"isenabled": true}).All(&companies.Data)
	return companies, err
}

// CreateIndexes - create necessary indexes for fast executing
func (cr *CompanyRepo) CreateIndexes() {
	c := cr.sess.C(cr.coll)
	c.EnsureIndex(mgo.Index{
		Key:    []string{"id"},
		Unique: true,
	})
}

// DeleteCompanyByID - set company as disabled from database by id
func (cr *CompanyRepo) DeleteCompanyByID(id string) error {
	c := cr.sess.C(cr.coll)
	err := c.Update(bson.M{"id": id}, bson.M{"$set": bson.M{"isenabled": false}})
	return err
}

// UpdateCompany - update company info by id
func (cr *CompanyRepo) UpdateCompany(company *grpc_gateway_company.Company) error {
	oldCompany, err := cr.GetCompanyByID(company.Id)
	if err != nil {
		return err
	}

	oldCompany.Name = company.Name
	c := cr.sess.C(cr.coll)
	err = c.Update(bson.M{"id": oldCompany.Id}, oldCompany)
	return err
}
