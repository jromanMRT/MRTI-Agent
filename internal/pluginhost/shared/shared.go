// Package shared defines the contract between the MRTI Agent (plugin host) and
// external collector plugins. Both the agent and every plugin binary import
// this package, guaranteeing they agree on the handshake, the Go interface and
// the gRPC adapters. This is the HashiCorp go-plugin pattern: plugins are
// separate processes, so a crashing or misbehaving plugin cannot take down the
// agent, and new plugins are added by dropping a binary in — no recompiling
// the core.
package shared

import (
	"context"

	"github.com/hashicorp/go-plugin"
	pb "github.com/jromanMRT/mrti-agent/proto/collectorpb"
	"google.golang.org/grpc"
)

// Handshake is a mutual sanity check: a plugin launched without the matching
// magic cookie exits immediately rather than misbehaving. Bump ProtocolVersion
// on breaking changes to the Collector service.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "MRTI_PLUGIN",
	MagicCookieValue: "mrti-collector-v1",
}

// PluginName is the key under which the collector plugin is dispensed.
const PluginName = "collector"

// Collector is the interface a plugin author implements. It is deliberately
// small and JSON-based so plugins evolve independently of the agent.
type Collector interface {
	// Info returns name, version and a human description.
	Info() (name, version, description string)
	// Configure receives the plugin's settings as JSON (may be empty).
	Configure(settingsJSON []byte) error
	// Collect performs one collection and returns a JSON payload.
	Collect() (dataJSON []byte, err error)
}

// PluginClient is the host-facing view of a running plugin. Unlike Collector
// it carries a context so the agent can bound calls with timeouts.
type PluginClient interface {
	Info(ctx context.Context) (name, version, description string, err error)
	Configure(ctx context.Context, settingsJSON []byte) error
	Collect(ctx context.Context) (dataJSON []byte, err error)
}

// PluginMap is passed to both the host (client) and the plugin (serve) sides.
func PluginMap(impl Collector) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		PluginName: &CollectorPlugin{Impl: impl},
	}
}

// CollectorPlugin wires the go-plugin machinery for the Collector service.
type CollectorPlugin struct {
	plugin.NetRPCUnsupportedPlugin // gRPC only; refuse net/rpc
	Impl                           Collector
}

// GRPCServer registers the plugin implementation on the plugin process side.
func (p *CollectorPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterCollectorServer(s, &grpcServer{impl: p.Impl})
	return nil
}

// GRPCClient returns the host-side adapter implementing PluginClient.
func (p *CollectorPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &grpcClient{client: pb.NewCollectorClient(c)}, nil
}

// --- server adapter: Collector -> pb.CollectorServer ---

type grpcServer struct {
	pb.UnimplementedCollectorServer
	impl Collector
}

func (s *grpcServer) Info(_ context.Context, _ *pb.Empty) (*pb.PluginInfo, error) {
	n, v, d := s.impl.Info()
	return &pb.PluginInfo{Name: n, Version: v, Description: d}, nil
}

func (s *grpcServer) Configure(_ context.Context, req *pb.ConfigureRequest) (*pb.Empty, error) {
	if err := s.impl.Configure(req.GetSettingsJson()); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *grpcServer) Collect(_ context.Context, _ *pb.Empty) (*pb.CollectResponse, error) {
	data, err := s.impl.Collect()
	resp := &pb.CollectResponse{DataJson: data}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, nil
}

// --- client adapter: pb.CollectorClient -> PluginClient ---

type grpcClient struct {
	client pb.CollectorClient
}

func (c *grpcClient) Info(ctx context.Context) (string, string, string, error) {
	info, err := c.client.Info(ctx, &pb.Empty{})
	if err != nil {
		return "", "", "", err
	}
	return info.GetName(), info.GetVersion(), info.GetDescription(), nil
}

func (c *grpcClient) Configure(ctx context.Context, settingsJSON []byte) error {
	_, err := c.client.Configure(ctx, &pb.ConfigureRequest{SettingsJson: settingsJSON})
	return err
}

func (c *grpcClient) Collect(ctx context.Context) ([]byte, error) {
	resp, err := c.client.Collect(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}
	if resp.GetError() != "" {
		return resp.GetDataJson(), &PluginError{Msg: resp.GetError()}
	}
	return resp.GetDataJson(), nil
}

// PluginError wraps an error string returned by a plugin's Collect.
type PluginError struct{ Msg string }

func (e *PluginError) Error() string { return e.Msg }

// Serve is a convenience for plugin main() functions.
func Serve(impl Collector) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap(impl),
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
