#!/bin/bash

if [[ -z $1 ]];  then
    echo "You should specify file with proto-description";
    exit 0;
fi

protoc -I/usr/local/include -I.  -I$GOPATH/src  -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis  --go_out=Mgoogle/api/annotations.proto=github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis/google/api,plugins=grpc:. $1
protoc -I/usr/local/include -I.  -I$GOPATH/src  -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis  --grpc-gateway_out=logtostderr=true:.  $1
protoc -I/usr/local/include -I.  -I$GOPATH/src  -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis  --swagger_out=logtostderr=true:. $1
for i in {"entity",}
do
  echo $i
  sed -i ''  's|,omitempty||'  proto/$i/$i.pb.go
  sed -i ''  's|proto/common|git.simplendi.com/FirmQ/frontend-server/server/proto/common|'  proto/$i/$i.pb.go
  sed -i ''  's|proto/common|git.simplendi.com/FirmQ/frontend-server/server/proto/common|'  proto/$i/$i.pb.gw.go
done
