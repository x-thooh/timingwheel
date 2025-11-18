package validate

//go:generate protoc -I . --go_out=. --go_opt=paths=source_relative --go-grpc_out=. validate_ext.proto
