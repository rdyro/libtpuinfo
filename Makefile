LIBNAME="libtpuinfo"
DYLIB_EXT := so

all:
	echo "nothing to be done"
	
clean:
	rm -rf lib/*
	rm -f ${LIBNAME}

lib:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
	CC="zig cc -target x86_64-linux-gnu -static" \
	CXX="zig c++ -target x86_64-linux-gnu -static" \
	LDFLAGS="-target x86_64-linux-gnu -shared" \
	go build -buildmode=c-shared -o lib/${LIBNAME}-linux-x86_64.${DYLIB_EXT}

	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
	CC="zig cc -target aarch64-linux-gnu -static" \
	CXX="zig c++ -target aarch64-linux-gnu -static" \
	LDFLAGS="-target aarch64-linux-gnu -shared" \
	go build -buildmode=c-shared -o lib/${LIBNAME}-linux-aarch64.${DYLIB_EXT}

	ln -s lib/${LIBNAME}-linux-x86_64.${DYLIB_EXT} ${LIBNAME}.${DYLIB_EXT}

cmain:
	gcc c_calling/main.c -ldl -o lib/cmain
	 ./lib/cmain


# development only #############################################################

regenerate_proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. \
	--go-grpc_opt=paths=source_relative tpu_info_proto/tpu_metric_service.proto

install_grpc:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

.PHONY: lib regenerate_proto install_grpc cmain lib clean
