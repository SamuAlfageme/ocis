SHELL := bash
NAME := thumbnails

############ tooling ############
include ../.bingo/Variables.mk

############ go tooling ############
include ../.make/go.mk

############ release #########
include ../.make/release.mk

############ docs generate ############
include ../.make/docs.mk

.PHONY: docs-generate
docs-generate: config-docs-generate \
			   grpc-docs-generate

############ generate ############
include ../.make/generate.mk

.PHONY: ci-go-generate
ci-go-generate: protobuf # CI runs ci-node-generate automatically before this target
	@go generate $(GENERATE)

.PHONY: ci-node-generate
ci-node-generate:

############ protobuf ############
include ../.make/protobuf.mk

PROTO_VERSION := v0
PROTO_SRC := pkg/proto/$(PROTO_VERSION)

.PHONY: protobuf
protobuf: $(PROTO_SRC)/${NAME}.pb.go \
		  $(PROTO_SRC)/${NAME}.pb.micro.go \
		  $(PROTO_SRC)/${NAME}.swagger.json \
		  ../docs/extensions/${NAME}/grpc.md
		  #$(PROTO_SRC)/${NAME}.pb.web.go \ # not needed for thumbnails
