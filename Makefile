all:
	echo "nothing to be done"
	
clean:
	rm -rf lib/*
	rm -f tpu_info_lib

lib:
	go build -buildmode=c-shared -o lib/tpu_info_lib.so

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
