package tunnel

import (
	"context"
	"errors"
	"testing"

	"github.com/lucasile/deft/internal/proto"
	"google.golang.org/grpc"
)

type mockStream struct {
	grpc.ClientStream
	sentMessages []*proto.AgentMessage
}

func TestHandler_HandleCommand_Shutdown(t *testing.T) {
	mock := &mockStream{}
	conn := &Connection{
		stream: mock,
	}
	handler := NewHandler(nil, conn, "test-node")

	cmd := &proto.PanelCommand{
		CommandId: "shutdown-1",
		Action: &proto.PanelCommand_Shutdown{
			Shutdown: &proto.ShutdownAgent{},
		},
	}

	err := handler.HandleCommand(context.Background(), cmd)
	if !errors.Is(err, ErrShutdownRequested) {
		t.Fatalf("HandleCommand error = %v, want ErrShutdownRequested", err)
	}

	if len(mock.sentMessages) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(mock.sentMessages))
	}

	res := mock.sentMessages[0].GetResult()
	if res == nil {
		t.Fatal("Sent message content is not a result")
	}
	if !res.Success {
		t.Fatalf("shutdown result success = false, message = %q", res.Message)
	}
	if res.CommandId != "shutdown-1" {
		t.Fatalf("command ID = %q, want shutdown-1", res.CommandId)
	}
	if res.Message != "Agent stopping" {
		t.Fatalf("message = %q, want Agent stopping", res.Message)
	}
}

func (m *mockStream) Send(msg *proto.AgentMessage) error {
	m.sentMessages = append(m.sentMessages, msg)
	return nil
}

func (m *mockStream) Recv() (*proto.PanelCommand, error) {
	return nil, nil
}

func TestHandler_HandleCommand_Unknown(t *testing.T) {
	mock := &mockStream{}
	conn := &Connection{
		stream: mock,
	}
	handler := NewHandler(nil, conn, "test-node")

	cmd := &proto.PanelCommand{
		CommandId: "123",
		// Action is nil
	}

	err := handler.HandleCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("HandleCommand failed: %v", err)
	}

	if len(mock.sentMessages) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(mock.sentMessages))
	}

	res := mock.sentMessages[0].GetResult()
	if res == nil {
		t.Fatal("Sent message content is not a result")
	}

	if res.Success {
		t.Error("Expected failure for unknown command, got success")
	}

	if res.CommandId != "123" {
		t.Errorf("Expected command ID 123, got %s", res.CommandId)
	}
}
