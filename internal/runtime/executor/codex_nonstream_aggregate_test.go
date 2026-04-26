package executor

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestAggregateCodexNonStreamCompletion_RebuildsMessageOutput(t *testing.T) {
	events := [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","created_at":1777190400,"output":[]}}`),
		[]byte(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"delta":"O"}`),
		[]byte(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"delta":"K"}`),
		[]byte(`{"type":"response.output_text.done","item_id":"msg_1","output_index":0,"text":"OK"}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.4","created_at":1777190400,"status":"completed","usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12},"output":[]}}`),
	}

	out, ok := aggregateCodexNonStreamCompletion(events)
	if !ok {
		t.Fatalf("expected aggregation to succeed")
	}

	if got := gjson.GetBytes(out, "response.output.0.type").String(); got != "message" {
		t.Fatalf("output[0].type = %q, want %q body=%s", got, "message", string(out))
	}
	if got := gjson.GetBytes(out, "response.output.0.content.0.text").String(); got != "OK" {
		t.Fatalf("output text = %q, want %q body=%s", got, "OK", string(out))
	}
	if got := gjson.GetBytes(out, "response.usage.total_tokens").Int(); got != 12 {
		t.Fatalf("usage.total_tokens = %d, want 12 body=%s", got, string(out))
	}
}

func TestAggregateCodexNonStreamCompletion_RebuildsFunctionCallOutput(t *testing.T) {
	events := [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_2","model":"gpt-5.4","created_at":1777190401,"output":[]}}`),
		[]byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_call_1","type":"function_call","status":"in_progress","call_id":"call_1","name":"web_search"}}`),
		[]byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_call_1","output_index":0,"delta":"{\"query\":\"hel"}`),
		[]byte(`{"type":"response.function_call_arguments.done","item_id":"fc_call_1","output_index":0,"arguments":"{\"query\":\"hello\"}"}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_2","model":"gpt-5.4","created_at":1777190401,"status":"completed","output":[]}}`),
	}

	out, ok := aggregateCodexNonStreamCompletion(events)
	if !ok {
		t.Fatalf("expected aggregation to succeed")
	}

	if got := gjson.GetBytes(out, "response.output.0.type").String(); got != "function_call" {
		t.Fatalf("output[0].type = %q, want %q body=%s", got, "function_call", string(out))
	}
	if got := gjson.GetBytes(out, "response.output.0.call_id").String(); got != "call_1" {
		t.Fatalf("call_id = %q, want %q body=%s", got, "call_1", string(out))
	}
	if got := gjson.GetBytes(out, "response.output.0.name").String(); got != "web_search" {
		t.Fatalf("name = %q, want %q body=%s", got, "web_search", string(out))
	}
	if got := gjson.GetBytes(out, "response.output.0.arguments").String(); got != `{"query":"hello"}` {
		t.Fatalf("arguments = %q, want %q body=%s", got, `{"query":"hello"}`, string(out))
	}
}

func TestAggregateCodexNonStreamCompletion_PreservesReasoningBeforeMessage(t *testing.T) {
	events := [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_3","model":"gpt-5.4","created_at":1777190402,"output":[]}}`),
		[]byte(`{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","output_index":0,"delta":"Let me "}`),
		[]byte(`{"type":"response.reasoning_summary_text.done","item_id":"rs_1","output_index":0,"text":"Let me think"}`),
		[]byte(`{"type":"response.output_text.done","item_id":"msg_1","output_index":1,"text":"OK"}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_3","model":"gpt-5.4","created_at":1777190402,"status":"completed","output":[]}}`),
	}

	out, ok := aggregateCodexNonStreamCompletion(events)
	if !ok {
		t.Fatalf("expected aggregation to succeed")
	}

	if got := gjson.GetBytes(out, "response.output.0.type").String(); got != "reasoning" {
		t.Fatalf("output[0].type = %q, want %q body=%s", got, "reasoning", string(out))
	}
	if got := gjson.GetBytes(out, "response.output.0.summary.0.text").String(); got != "Let me think" {
		t.Fatalf("reasoning text = %q, want %q body=%s", got, "Let me think", string(out))
	}
	if got := gjson.GetBytes(out, "response.output.1.type").String(); got != "message" {
		t.Fatalf("output[1].type = %q, want %q body=%s", got, "message", string(out))
	}
	if got := gjson.GetBytes(out, "response.output.1.content.0.text").String(); got != "OK" {
		t.Fatalf("message text = %q, want %q body=%s", got, "OK", string(out))
	}
}

func TestAggregateCodexNonStreamCompletion_PreservesOriginalUnknownOutputItems(t *testing.T) {
	events := [][]byte{
		[]byte(`{"type":"response.completed","response":{"id":"resp_4","model":"gpt-5.4","created_at":1777190403,"status":"completed","output":[{"id":"foo_1","type":"function_call_output","call_id":"call_1","output":"done"}]}}`),
	}

	out, ok := aggregateCodexNonStreamCompletion(events)
	if !ok {
		t.Fatalf("expected aggregation to succeed")
	}

	if got := gjson.GetBytes(out, "response.output.0.type").String(); got != "function_call_output" {
		t.Fatalf("output[0].type = %q, want %q body=%s", got, "function_call_output", string(out))
	}
	if got := gjson.GetBytes(out, "response.output.0.output").String(); got != "done" {
		t.Fatalf("output = %q, want %q body=%s", got, "done", string(out))
	}
}
