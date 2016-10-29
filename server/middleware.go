package server

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net/http"
	"path"
	"strings"
)

var secretKey = []byte(`lc}m8K!0.-{3m8y%VJL!eFAc!S2:673<eN29Cod1c9q9dRH6Wa|&//l^6tt4hq3`)

func isPathRequriredAuthorization(info *grpc.UnaryServerInfo) bool {
	return info.FullMethod != "/grpc.gateway.user.UserService/Login" &&
		info.FullMethod != "/grpc.gateway.user.UserService/ConfirmEmail"
}

// AuthUnaryInterceptor - interceptor function
// https://godoc.org/google.golang.org/grpc#UnaryServerInterceptor
func AuthUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {

	if isPathRequriredAuthorization(info) {
		errMessage := NewCommonResponse()
		errMessage.Meta.Ok = false
		errMessage.Meta.Error = "authentication required"
		errMessage.Meta.StatusCode = http.StatusUnauthorized

		// retrieve metadata from context
		md, ok := metadata.FromContext(ctx)
		if !ok {
			return errMessage, nil
		}

		if _, ok := md["authorization"]; !ok || len(md["authorization"]) < 0 {
			return errMessage, nil
		}

		authorizationParts := strings.Split(md["authorization"][0], " ")
		if len(authorizationParts) < 2 {
			return errMessage, nil
		}

		tokenString := authorizationParts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Don't forget to validate the alg is what you expect:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				errMessage.Meta.Error = fmt.Sprintf("Unexpected signing method: %v", token.Header["alg"])
				return errMessage, nil
			}

			return secretKey, nil
		})

		if err != nil {
			return errMessage, nil
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID := claims["user_id"].(string)
			ctx = context.WithValue(ctx, "user_id", userID)
		} else {
			return errMessage, nil
		}
	}

	return handler(ctx, req)
}

func serveSwagger(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, ".swagger.json") {
		glog.Errorf("Not Found: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	p := strings.TrimPrefix(r.URL.Path, "/swagger/")
	p = path.Join("proto", p)
	http.ServeFile(w, r, p)
}

func allowCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
				preflightHandler(w, r)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

func preflightHandler(w http.ResponseWriter, r *http.Request) {
	headers := []string{"Content-Type", "Accept", "Authorization"}
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(headers, ","))
	methods := []string{"GET", "HEAD", "POST", "PUT", "DELETE"}
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ","))
	return
}
