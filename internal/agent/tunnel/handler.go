package tunnel

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/lucasile/deft/internal/agent/docker"
	"github.com/lucasile/deft/internal/agent/docker/console"
	dockercontainer "github.com/lucasile/deft/internal/agent/docker/container"
	"github.com/lucasile/deft/internal/proto"
	"github.com/rs/zerolog/log"
)

var ErrShutdownRequested = errors.New("agent shutdown requested")

type Handler struct {
	docker      *docker.Client
	conn        *Connection
	nodeID      string
	logStreams  map[string]*logStream
	logStreamMu sync.Mutex
}

type logStream struct {
	cancel context.CancelFunc
}

func NewHandler(docker *docker.Client, conn *Connection, nodeID string) *Handler {
	return &Handler{
		docker:     docker,
		conn:       conn,
		nodeID:     nodeID,
		logStreams: make(map[string]*logStream),
	}
}

func (h *Handler) HandleCommand(ctx context.Context, cmd *proto.PanelCommand) error {
	var err error
	var msg string

	switch a := cmd.Action.(type) {
	case *proto.PanelCommand_Create:
		displayName := a.Create.DisplayName
		if displayName == "" {
			displayName = a.Create.Name
		}
		containerConfig := &container.Config{
			Image: a.Create.Image,
			Labels: map[string]string{
				dockercontainer.LabelManaged:    "true",
				dockercontainer.LabelNodeID:     h.nodeID,
				dockercontainer.LabelName:       displayName,
				dockercontainer.LabelResourceID: a.Create.ResourceId,
			},
			Env:          dockerEnv(a.Create.Env),
			ExposedPorts: dockerExposedPorts(a.Create.Ports),
		}
		hostConfig := &container.HostConfig{
			PortBindings:  dockerPortBindings(a.Create.Ports),
			Binds:         dockerBinds(a.Create.Volumes),
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyMode(a.Create.RestartPolicy)},
		}
		_, err = dockercontainer.Create(ctx, h.docker, a.Create.Name, a.Create.Image, containerConfig, hostConfig)
		msg = "Container created"
	case *proto.PanelCommand_Start:
		err = dockercontainer.Start(ctx, h.docker, a.Start.Id)
		msg = "Container started"
	case *proto.PanelCommand_Stop:
		err = dockercontainer.Stop(ctx, h.docker, a.Stop.Id)
		msg = "Container stopped"
	case *proto.PanelCommand_Remove:
		err = dockercontainer.Remove(ctx, h.docker, a.Remove.Id)
		msg = "Container removed"
	case *proto.PanelCommand_Restart:
		err = dockercontainer.Restart(ctx, h.docker, a.Restart.Id)
		msg = "Container restarted"
	case *proto.PanelCommand_Shutdown:
		msg = "Agent stopping"
	case *proto.PanelCommand_Logs:
		logs, logsErr := console.FetchLogs(ctx, h.docker, a.Logs.Id, int(a.Logs.TailLines))
		if logsErr != nil {
			err = logsErr
		} else if logs == "" {
			msg = "No logs available."
		} else {
			msg = logs
		}
	case *proto.PanelCommand_FollowLogs:
		if streamErr := h.startLogStream(ctx, a.FollowLogs.StreamId, a.FollowLogs.Id, int(a.FollowLogs.TailLines)); streamErr != nil {
			_ = h.sendLogChunk(a.FollowLogs.StreamId, a.FollowLogs.Id, nil, true, streamErr.Error())
		}
		return nil
	case *proto.PanelCommand_CancelLogs:
		h.cancelLogStream(a.CancelLogs.StreamId)
		return nil
	default:
		err = fmt.Errorf("unknown command type")
	}

	result := &proto.CommandResult{
		CommandId: cmd.CommandId,
		Success:   err == nil,
		Message:   msg,
	}

	if err != nil {
		result.Message = err.Error()
		log.Error().Err(err).Str("command_id", cmd.CommandId).Msg("Command failed")
	}

	_, shutdown := cmd.Action.(*proto.PanelCommand_Shutdown)
	if err == nil && !shutdown {
		if syncErr := h.SendContainerInventory(ctx); syncErr != nil {
			log.Error().Err(syncErr).Str("command_id", cmd.CommandId).Msg("failed to send container inventory")
		}
	}

	if sendErr := h.conn.SendMessage(&proto.AgentMessage{
		NodeId: h.nodeID,
		Content: &proto.AgentMessage_Result{
			Result: result,
		},
	}); sendErr != nil {
		return sendErr
	}

	if shutdown && err == nil {
		return ErrShutdownRequested
	}

	return nil
}

func dockerEnv(items []*proto.EnvVar) []string {
	if len(items) == 0 {
		return nil
	}
	env := make([]string, 0, len(items))
	for _, item := range items {
		if item.GetKey() == "" {
			continue
		}
		env = append(env, item.GetKey()+"="+item.GetValue())
	}
	return env
}

func dockerExposedPorts(items []*proto.PortMapping) nat.PortSet {
	if len(items) == 0 {
		return nil
	}
	ports := nat.PortSet{}
	for _, item := range items {
		port := nat.Port(fmt.Sprintf("%d/%s", item.GetContainerPort(), item.GetProtocol()))
		ports[port] = struct{}{}
	}
	return ports
}

func dockerPortBindings(items []*proto.PortMapping) nat.PortMap {
	if len(items) == 0 {
		return nil
	}
	bindings := nat.PortMap{}
	for _, item := range items {
		port := nat.Port(fmt.Sprintf("%d/%s", item.GetContainerPort(), item.GetProtocol()))
		bindings[port] = append(bindings[port], nat.PortBinding{
			HostPort: fmt.Sprintf("%d", item.GetHostPort()),
		})
	}
	return bindings
}

func dockerBinds(items []*proto.VolumeMount) []string {
	if len(items) == 0 {
		return nil
	}
	binds := make([]string, 0, len(items))
	for _, item := range items {
		if item.GetHostPath() == "" || item.GetContainerPath() == "" {
			continue
		}
		mode := "rw"
		if item.GetReadOnly() {
			mode = "ro"
		}
		binds = append(binds, item.GetHostPath()+":"+item.GetContainerPath()+":"+mode)
	}
	return binds
}

func (h *Handler) startLogStream(ctx context.Context, streamID, containerID string, tailLines int) error {
	if streamID == "" {
		return fmt.Errorf("log stream id is required")
	}
	if containerID == "" {
		return fmt.Errorf("container id is required")
	}

	streamCtx, cancel := context.WithCancel(ctx)
	entry := &logStream{cancel: cancel}
	h.logStreamMu.Lock()
	if existing, ok := h.logStreams[streamID]; ok {
		existing.cancel()
	}
	h.logStreams[streamID] = entry
	h.logStreamMu.Unlock()

	go func() {
		defer func() {
			h.logStreamMu.Lock()
			if current, ok := h.logStreams[streamID]; ok && current == entry {
				delete(h.logStreams, streamID)
			}
			h.logStreamMu.Unlock()
		}()

		err := console.FollowLogs(streamCtx, h.docker, containerID, tailLines, func(chunk []byte) error {
			return h.sendLogChunk(streamID, containerID, chunk, false, "")
		})
		if errors.Is(streamCtx.Err(), context.Canceled) {
			err = nil
		}

		errorMessage := ""
		if err != nil {
			errorMessage = err.Error()
			log.Error().Err(err).Str("stream_id", streamID).Str("container_id", containerID).Msg("log stream failed")
		}
		if sendErr := h.sendLogChunk(streamID, containerID, nil, true, errorMessage); sendErr != nil {
			log.Error().Err(sendErr).Str("stream_id", streamID).Msg("failed to send log stream EOF")
		}
	}()

	return nil
}

func (h *Handler) cancelLogStream(streamID string) {
	if streamID == "" {
		return
	}

	h.logStreamMu.Lock()
	entry, ok := h.logStreams[streamID]
	if ok {
		delete(h.logStreams, streamID)
	}
	h.logStreamMu.Unlock()

	if ok {
		entry.cancel()
	}
}

func (h *Handler) sendLogChunk(streamID, containerID string, data []byte, eof bool, errorMessage string) error {
	return h.conn.SendMessage(&proto.AgentMessage{
		NodeId: h.nodeID,
		Content: &proto.AgentMessage_LogChunk{
			LogChunk: &proto.LogChunk{
				StreamId:    streamID,
				ContainerId: containerID,
				Data:        data,
				Eof:         eof,
				Error:       errorMessage,
			},
		},
	})
}

func (h *Handler) SendContainerInventory(ctx context.Context) error {
	containers, err := dockercontainer.ListManaged(ctx, h.docker, h.nodeID)
	if err != nil {
		return err
	}

	summaries := make([]*proto.ContainerSummary, 0, len(containers))
	for _, item := range containers {
		summaries = append(summaries, &proto.ContainerSummary{
			Id:         item.ID,
			Name:       item.Name,
			Image:      item.Image,
			Status:     item.Status,
			ResourceId: item.ResourceID,
		})
	}

	return h.conn.SendMessage(&proto.AgentMessage{
		NodeId: h.nodeID,
		Content: &proto.AgentMessage_Inventory{
			Inventory: &proto.ContainerInventory{
				Containers: summaries,
			},
		},
	})
}
