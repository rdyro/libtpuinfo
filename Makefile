LIBNAME := libtpuinfo
DYLIB_EXT := so
LIB=${LIBNAME}.${DYLIB_EXT}
LIB_X86_64=lib/${LIBNAME}-linux-x86_64.${DYLIB_EXT}
LIB_AARCH64=lib/${LIBNAME}-linux-aarch64.${DYLIB_EXT}
CC := gcc

${LIB}: main.go
	go build -buildmode=c-shared -o ${LIB} main.go
	rm -f ${LIBNAME}.h

install: ${LIB}
	cp ${LIB} /lib/
	
clean:
	rm -f ${LIBNAME}.h ${LIB} ${LIB_X86_64} ${LIB_AARCH64} lib/*

${LIB_X86_64}: main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
	CC="zig cc -target x86_64-linux-gnu -static" \
	CXX="zig c++ -target x86_64-linux-gnu -static" \
	LDFLAGS="-target x86_64-linux-gnu -shared" \
	go build -buildmode=c-shared -o ${LIB_X86_64}

${LIB_AARCH64}: main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
	CC="zig cc -target aarch64-linux-gnu -static" \
	CXX="zig c++ -target aarch64-linux-gnu -static" \
	LDFLAGS="-target aarch64-linux-gnu -shared" \
	go build -buildmode=c-shared -o ${LIB_AARCH64}

release: ${LIB_X86_64} ${LIB_AARCH64}
	# get version of targets with .h suffix and delete them
	rm -f $(patsubst %.so,%.h,${LIB_X86_64} ${LIB_AARCH64})

test: ${LIB}
	${CC} c_calling/main.c -ldl -o lib/cmain
	LD_LIBRARY_PATH=".:${LD_LIBRARY_PATH}" ./lib/cmain

# development only #############################################################

regenerate_proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. \
	--go-grpc_opt=paths=source_relative tpu_info_proto/tpu_metric_service.proto

install_grpc:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

.PHONY: clean install regenerate_proto install_grpc test release