#!/bin/bash

go test -v -cover ./server
golint ./server/
godep save