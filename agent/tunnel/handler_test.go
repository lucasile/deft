package tunnel

import (
	"context"
	"testing"

	"github.com/lucasile/deft/proto"
	"google.golang.org/grpc"
)

type mockStream struct {
	grpc.ClientStream
	sentMessages []*proto.AgentMessage
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
