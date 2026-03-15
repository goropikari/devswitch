package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"traefix/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// サービス実装
type server struct {
	proto.UnimplementedHelloServiceServer
}

var port string

func (s *server) SayHello(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	return &proto.HelloReply{Message: "Hello from port " + port}, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <port>")
		return
	}
	port = os.Args[1]
	addr := ":" + port

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterHelloServiceServer(grpcServer, &server{})

	// リフレクション有効化
	reflection.Register(grpcServer)
	log.Printf("gRPC server listening on %s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
