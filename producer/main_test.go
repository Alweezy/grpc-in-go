package main

import (
	"context"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"grpc-in-go/pb"
	"net"
	"testing"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

// Setup a buffered connection for gRPC server-client testing
func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

// TestTaskGeneration validates that the task is generated and sent over gRPC.
func TestTaskGeneration(t *testing.T) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	pb.RegisterTaskServiceServer(s, &mockTaskServer{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Fatalf("Server failed to start: %v", err)
		}
	}()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	assert.NoError(t, err)
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	client := pb.NewTaskServiceClient(conn)

	// Simulate a task being sent
	res, err := client.SendTask(ctx, &pb.TaskRequest{Type: 2, Value: 50})
	assert.NoError(t, err)
	assert.Equal(t, "Processed", res.Status)
}

// Mock gRPC Task Server
type mockTaskServer struct {
	pb.UnimplementedTaskServiceServer
}

func (s *mockTaskServer) SendTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	return &pb.TaskResponse{Status: "Processed"}, nil
}
