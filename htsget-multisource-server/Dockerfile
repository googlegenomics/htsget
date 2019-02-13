# Copyright 2017 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.11 AS SRC

WORKDIR /go/src/github.com/googlegenomics/htsget
COPY . .
ENV GO111MODULE=on
RUN go build htsget-multisource-server/main.go 


FROM ubuntu:19.04

WORKDIR /app
COPY --from=SRC  /go/src/github.com/googlegenomics/htsget/main /app/

COPY --from=SRC  /go/src/github.com/googlegenomics/htsget/htsget-multisource-server/file/testdata /app/data

EXPOSE 8080

# By default, the server listens for plain HTTP requests on the default port
# (exposed above) and serves requests to public data only.  See the README for
# information about setting up secure access and other supported features.
CMD ["/app/main", "-directory", "/app/data"]
