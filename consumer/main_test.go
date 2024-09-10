package main

import (
	"context"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
	"yq-app-challenge/pb"
)

// 1MB buffer size for the listener
const bufSize = 1024 * 1024

var lis *bufconn.Listener

// bufDialer function to dial the in-memory gRPC server using bufconn
func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial() // Use the global listener `lis` for dialing
}

// MockTaskServiceServer for testing
type mockTaskServiceServer struct {
	pb.UnimplementedTaskServiceServer
	limiter *rate.Limiter
}

func (s *mockTaskServiceServer) SendTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	if err := s.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Simulate processing delay based on task value
	time.Sleep(time.Duration(req.Value) * time.Millisecond)
	return &pb.TaskResponse{Status: "Processed"}, nil
}

// TestConsumerRateLimiting validates rate-limited task processing
func TestConsumerRateLimiting(t *testing.T) {
	lis = bufconn.Listen(bufSize) // Initialize bufconn listener
	s := grpc.NewServer()
	limiter := rate.NewLimiter(1, 1) // 1 task per second for testing
	pb.RegisterTaskServiceServer(s, &mockTaskServiceServer{limiter: limiter})

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

	// Simulate sending a task with a value of 500 (should process after 500ms)
	start := time.Now()
	res, err := client.SendTask(ctx, &pb.TaskRequest{Type: 3, Value: 500})
	assert.NoError(t, err)
	assert.Equal(t, "Processed", res.Status)
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(500)) // Ensure the delay occurred
}
