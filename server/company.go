package server

import (
	grpc_gateway_common "git.simplendi.com/FirmQ/frontend-server/server/proto/common"
	grpc_gateway_company "git.simplendi.com/FirmQ/frontend-server/server/proto/company"
	"golang.org/x/net/context"
	//"google.golang.org/grpc/metadata"
	google_protobuf1 "github.com/golang/protobuf/ptypes/empty"
	"gopkg.in/mgo.v2"
	"net/http"
)

type companyServer struct {
	sess *mgo.Session
}

// NewCompanyResponse - create new instance of company response
func NewCompanyResponse() *grpc_gateway_company.CompanyResponse {
	message := &grpc_gateway_company.CompanyResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	return message
}

// NewCompanyListResponse - create new instance of company list response
func NewCompanyListResponse() *grpc_gateway_company.CompanyListResponse {
	message := &grpc_gateway_company.CompanyListResponse{}
	message.Meta = &grpc_gateway_common.MetaResponse{StatusCode: http.StatusOK}
	return message
}

// NewCompanyServer - returns new grpc server which provide company-related functionality
func NewCompanyServer() grpc_gateway_company.CompanyServiceServer {
	return new(companyServer)
}

func (c *companyServer) CreateCompany(ctx context.Context, company *grpc_gateway_company.Company) (*grpc_gateway_common.IDResponse, error) {
	message := NewIDResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	companyRepo := NewCompanyRepo(sess)

	if err := IsAdminUser(ctx); err != nil {
		message.Meta.StatusCode = http.StatusForbidden
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	err = companyRepo.CreateCompany(company)
	if err == nil {
		message.Meta.Ok = true
		message.Id = company.Id
	} else {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
	}

	return message, nil
}

func (c *companyServer) UpdateCompany(ctx context.Context, company *grpc_gateway_company.Company) (*grpc_gateway_common.CommonResponse, error) {
	message := NewCommonResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	// check permissions. only admin user has access
	if err := IsAdminUser(ctx); err != nil {
		message.Meta.StatusCode = http.StatusForbidden
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	companyRepo := NewCompanyRepo(sess)
	err = companyRepo.UpdateCompany(company)
	if err == nil {
		message.Meta.Ok = true
	} else {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
	}

	return message, nil
}

func (c *companyServer) GetCompany(ctx context.Context, in *grpc_gateway_common.IDRequest) (*grpc_gateway_company.CompanyResponse, error) {
	company := NewCompanyResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		company.Meta.Ok = false
		company.Meta.Error = err.Error()
		return company, nil
	}

	companyRepo := NewCompanyRepo(sess)

	if err := IsAdminOrCompanyUser(ctx, in.Id); err != nil {
		company.Meta.Ok = false
		company.Meta.Error = err.Error()
		company.Meta.StatusCode = http.StatusForbidden
		return company, nil
	}

	company.Data, err = companyRepo.GetCompanyByID(in.Id)

	if err == mgo.ErrNotFound {
		company.Meta.Ok = false
		company.Meta.Error = ErrNotFound.Error()
		company.Meta.StatusCode = http.StatusNotFound
		return company, nil
	}

	company.Meta.Ok = true
	return company, nil
}

func (c *companyServer) GetCompanies(ctx context.Context, in *google_protobuf1.Empty) (*grpc_gateway_company.CompanyListResponse, error) {
	companyList := NewCompanyListResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		companyList.Meta.Ok = false
		companyList.Meta.Error = err.Error()
		return companyList, nil
	}

	companyRepo := NewCompanyRepo(sess)

	if err := IsAdminUser(ctx); err != nil {
		companyList.Meta.StatusCode = http.StatusForbidden
		companyList.Meta.Ok = false
		companyList.Meta.Error = err.Error()
		return companyList, nil
	}

	companyList, err = companyRepo.GetCompanies()
	companyList.Meta.Ok = true
	return companyList, nil
}

func (c *companyServer) DeleteCompany(ctx context.Context, in *grpc_gateway_common.IDRequest) (*grpc_gateway_common.CommonResponse, error) {
	message := NewCommonResponse()

	sess, err := connectionPoolInstance.GetConnection()
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	companyRepo := NewCompanyRepo(sess)

	if err := IsAdminUser(ctx); err != nil {
		message.Meta.StatusCode = http.StatusForbidden
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
		return message, nil
	}

	err = companyRepo.DeleteCompanyByID(in.Id)
	if err != nil {
		message.Meta.Ok = false
		message.Meta.Error = err.Error()
	} else {
		message.Meta.Ok = true
	}

	return message, nil
}
