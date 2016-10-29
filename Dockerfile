FROM alpine:3.5

ADD frontend-server /opt/bin/frontend-server/
ADD *.swagger.json /opt/bin/frontend-server/proto/
ADD index.html /opt/bin/frontend-server/

RUN ls -lah .
RUN ls -lah /opt/bin/frontend-server/
RUN apk --update add ca-certificates

WORKDIR "/opt/bin/frontend-server/"
ENTRYPOINT ["/opt/bin/frontend-server/frontend-server"]



