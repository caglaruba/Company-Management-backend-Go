package server

import (
	"bytes"
	"fmt"
	grpc_gateway_company "git.simplendi.com/FirmQ/frontend-server/server/proto/company"
	grpc_gateway_entity "git.simplendi.com/FirmQ/frontend-server/server/proto/entity"
	grpc_gateway_user "git.simplendi.com/FirmQ/frontend-server/server/proto/user"
	"github.com/golang/glog"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/philips/go-bindata-assetfs"
	"gitlab.com/grpc-gateway-example/pkg/ui/data/swagger"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// Config - type for server configuration
type Config struct {
	EmailUsername  string
	EmailPassword  string
	EmailFrom      string
	ServerURL      string
	EmailSMTP      string
	EmailSMTPPort  int
	NexmoAPIKey    string
	NexmoSecretKey string
	Port           string

	EmailConfirmationTTL time.Duration
	SMSConfirmationTTL   time.Duration
}

// Server - type of main server which provide this service
type Server struct {
	grpcServer *grpc.Server
	Config     *Config
}

// GetHTTPClient - return default (for this service) http client
func GetHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Minute,
	}
}

// NewServer - return new instance of Server
func NewServer(cfg *Config) (*Server, error) {
	s := Server{
		Config: cfg,
	}
	return &s, nil
}

func (s *Server) runGRPCServer() error {
	s.configureGRPCServer()

	l, err := net.Listen("tcp", ":9090")
	if err != nil {
		return err
	}

	go s.grpcServer.Serve(l)
	return nil
}

func (s *Server) configureGRPCServer() {
	s.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(AuthUnaryInterceptor))

	userServiceServer := NewUserServer(s.Config)
	grpc_gateway_user.RegisterUserServiceServer(s.grpcServer, userServiceServer)

	grpc_gateway_company.RegisterCompanyServiceServer(s.grpcServer, NewCompanyServer())

	grpc_gateway_entity.RegisterEntityServiceServer(s.grpcServer, NewEntityServer())

	// create default user
	if err := userServiceServer.(*userServer).createDefaultUser(); err != nil {
		glog.Error(err)
	} else {
		glog.Info("Default user created")
	}
}

// RunServer - starts all required functions to move server to working (active) state
func (s *Server) RunServer() error {
	connectionPoolInstance = NewConnectionPool()
	smsGatewayInstance = NewSMSGateway(s.Config.NexmoAPIKey, s.Config.NexmoSecretKey)
	emailInstance = NewEmailSender(s.Config)

	if err := s.runGRPCServer(); err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	grpcMux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{OrigName: true, EmitDefaults: true}))

	//grpcMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err := grpc_gateway_user.RegisterUserServiceHandlerFromEndpoint(ctx, grpcMux, ":9090", opts)
	if err != nil {
		return err
	}

	err = grpc_gateway_company.RegisterCompanyServiceHandlerFromEndpoint(ctx, grpcMux, ":9090", opts)
	if err != nil {
		return err
	}

	err = grpc_gateway_entity.RegisterEntityServiceHandlerFromEndpoint(ctx, grpcMux, ":9090", opts)
	if err != nil {
		return err
	}

	glog.Info("RPC-services started")
	mux := http.NewServeMux()

	// set up serving *.swagger.json files - files include swagger definition of our api services
	mux.HandleFunc("/swagger/", serveSwagger)

	// set up call which render swagger UI
	mux.HandleFunc("/swagger-ui/", func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/swagger-ui/" || r.RequestURI == "/swagger-ui/index.html" {
			data, err := ioutil.ReadFile("./index.html")
			if err != nil {
				fmt.Fprint(w, err)
			}
			http.ServeContent(w, r, "index.html", time.Now(), bytes.NewReader(data))
		} else {
			fileServer := http.StripPrefix("/swagger-ui/", http.FileServer(&assetfs.AssetFS{
				Asset:    swagger.Asset,
				AssetDir: swagger.AssetDir,
				Prefix:   "third_party/swagger-ui",
			}))
			fileServer.ServeHTTP(w, r)
		}
	})

	mux.Handle("/", grpcMux)

	return http.ListenAndServe(":8080", allowCORS(mux))
}
