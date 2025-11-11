module github.com/kamalyes/go-rpc-gateway/examples/02-ecommerce-service

go 1.21

require (
	github.com/kamalyes/go-core v0.0.1
	github.com/kamalyes/go-rpc-gateway v0.0.1
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.31.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.0
	google.golang.org/genproto v0.0.0-20231016165738-49dd2c1f3d0b
	google.golang.org/genproto/googleapis/api v0.0.0-20231016165738-49dd2c1f3d0b
)

// 本地依赖替换
replace github.com/kamalyes/go-rpc-gateway => ../../

replace github.com/kamalyes/go-core => ../../../go-core
