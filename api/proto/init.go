package proto

//go:generate protoc -I . --go_out=../ --go_opt=paths=source_relative --go-grpc_out=../ --go-grpc_opt=paths=source_relative --grpc-gateway_out=../ --grpc-gateway_opt=logtostderr=true --grpc-gateway_opt=paths=source_relative --validate_out=paths=source_relative,lang=go:../ delay/delay.proto

//go:generate protoc -I . --go_out=../ --go_opt=paths=source_relative --go-grpc_out=../ --go-grpc_opt=paths=source_relative --grpc-gateway_out=../ --grpc-gateway_opt=logtostderr=true --grpc-gateway_opt=paths=source_relative --validate_out=paths=source_relative,lang=go:../ example/example.proto
