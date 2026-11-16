#!/bin/sh

curl -1sLf 'https://dl.cloudsmith.io/public/task/task/setup.deb.sh' | sudo -E bash
sudo apt install task

export PATH="$(go env GOPATH)/bin:$PATH"
go install golang.org/x/tools/gopls@latest

