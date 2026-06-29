package ageval

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	sigilv1 "go.k6.io/k6/v2/internal/js/modules/k6/experimental/ageval/sigilv1"
)

func assistantToolCall(id, name, inputJSON string) *sigilv1.Message {
	return &sigilv1.Message{
		Role: sigilv1.MessageRole_MESSAGE_ROLE_ASSISTANT,
		Parts: []*sigilv1.Part{{
			Payload: &sigilv1.Part_ToolCall{ToolCall: &sigilv1.ToolCall{
				Id: id, Name: name, InputJson: []byte(inputJSON),
			}},
		}},
	}
}

func assistantText(text string) *sigilv1.Message {
	return &sigilv1.Message{
		Role:  sigilv1.MessageRole_MESSAGE_ROLE_ASSISTANT,
		Parts: []*sigilv1.Part{{Payload: &sigilv1.Part_Text{Text: text}}},
	}
}

func toolResult(id, content string) *sigilv1.Message {
	return &sigilv1.Message{
		Role: sigilv1.MessageRole_MESSAGE_ROLE_TOOL,
		Parts: []*sigilv1.Part{{
			Payload: &sigilv1.Part_ToolResult{ToolResult: &sigilv1.ToolResult{
				ToolCallId: id, Content: content,
			}},
		}},
	}
}

func TestSigilToTrajectory(t *testing.T) {
	t.Parallel()

	base := time.Unix(1_700_000_000, 0)

	// Two model round-trips: the first calls a tool, the second receives the
	// tool result (on its input) and emits the final answer. Deliberately
	// out of order to exercise the started_at sort.
	gen2 := &sigilv1.Generation{
		Id:        "g2",
		StartedAt: timestamppb.New(base.Add(time.Second)),
		Input:     []*sigilv1.Message{toolResult("t1", "18C, sunny")},
		Output:    []*sigilv1.Message{assistantText("It is 18C and sunny in Paris.")},
		Usage:     &sigilv1.TokenUsage{InputTokens: 100, OutputTokens: 20},
	}
	gen1 := &sigilv1.Generation{
		Id:        "g1",
		StartedAt: timestamppb.New(base),
		Output:    []*sigilv1.Message{assistantToolCall("t1", "get_weather", `{"city":"Paris"}`)},
		Usage:     &sigilv1.TokenUsage{InputTokens: 50, OutputTokens: 10, CacheReadInputTokens: 5},
	}

	tr := sigilToTrajectory([]*sigilv1.Generation{gen2, gen1})

	require.Len(t, tr.toolCalls, 1)
	assert.Equal(t, "get_weather", tr.toolCalls[0].Name)
	assert.Equal(t, "Paris", tr.toolCalls[0].Input["city"])
	assert.Equal(t, "18C, sunny", tr.toolCalls[0].Output, "tool result should be matched back by id")
	assert.Equal(t, "It is 18C and sunny in Paris.", tr.output, "final assistant text is the output")
	assert.Equal(t, int64(155), tr.inTok, "input + cache_read summed across generations")
	assert.Equal(t, int64(30), tr.outTok)
}

func TestSigilToTrajectoryEmpty(t *testing.T) {
	t.Parallel()

	tr := sigilToTrajectory(nil)
	assert.Empty(t, tr.toolCalls)
	assert.Empty(t, tr.output)
	assert.Zero(t, tr.inTok)
	assert.Zero(t, tr.outTok)
}

func TestSigilServerRoutesByRunID(t *testing.T) {
	t.Parallel()

	srv, err := startSigilServer()
	require.NoError(t, err)

	coll := srv.register("run-A")
	defer srv.deregister("run-A")

	// A generation tagged for a different run must not land in run-A's collector.
	_, err = srv.ExportGenerations(t.Context(), &sigilv1.ExportGenerationsRequest{
		Generations: []*sigilv1.Generation{
			{Id: "x", Tags: map[string]string{"test_run_id": "run-A"}},
			{Id: "y", Tags: map[string]string{"test_run_id": "run-B"}},
		},
	})
	require.NoError(t, err)

	gens := coll.drain(10*time.Millisecond, 100*time.Millisecond)
	require.Len(t, gens, 1)
	assert.Equal(t, "x", gens[0].GetId())
}

// TestSigilServerOverGRPC exercises the full wire path: a real gRPC client
// (as the Sigil SDK is) dials the server's ephemeral address and exports a
// generation, which must reach the matching run collector.
func TestSigilServerOverGRPC(t *testing.T) {
	t.Parallel()

	srv, err := startSigilServer()
	require.NoError(t, err)

	coll := srv.register("wire-1")
	defer srv.deregister("wire-1")

	conn, err := grpc.NewClient(srv.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := sigilv1.NewGenerationIngestServiceClient(conn)
	resp, err := client.ExportGenerations(t.Context(), &sigilv1.ExportGenerationsRequest{
		Generations: []*sigilv1.Generation{{
			Id:     "g",
			Tags:   map[string]string{"test_run_id": "wire-1"},
			Output: []*sigilv1.Message{assistantText("done")},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetResults(), 1)
	assert.True(t, resp.GetResults()[0].GetAccepted())

	gens := coll.drain(10*time.Millisecond, time.Second)
	require.Len(t, gens, 1)
	assert.Equal(t, "done", sigilToTrajectory(gens).output)
}

func TestSigilServerSummaries(t *testing.T) {
	t.Parallel()

	srv, err := startSigilServer()
	require.NoError(t, err)

	assert.Empty(t, srv.summaries())

	srv.recordSummary(sigilRunSummary{
		runID: "r1", agent: "abt", generations: 2,
		toolCalls:   []ToolCall{{Name: "navigate"}, {Name: "report_step"}},
		inputTokens: 100, outputTokens: 20, output: "done",
	})
	srv.recordSummary(sigilRunSummary{runID: "r2", agent: "abt", generations: 1})

	got := srv.summaries()
	require.Len(t, got, 2)
	assert.Equal(t, "r1", got[0].runID)
	assert.Len(t, got[0].toolCalls, 2)
	assert.Equal(t, int64(100), got[0].inputTokens)
	assert.Equal(t, "r2", got[1].runID)
}
