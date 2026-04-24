package gemini

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertCodexResponseToGemini_StreamPartialImageAddsInlineData(t *testing.T) {
	ctx := context.Background()
	var param any

	out := ConvertCodexResponseToGemini(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.created","response":{"id":"resp_123","created_at":1700000000,"model":"gpt-5.4"}}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected response.created metadata chunk, got %d", len(out))
	}

	out = ConvertCodexResponseToGemini(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.image_generation_call.partial_image","item_id":"img_1","partial_image_b64":"YWJj","output_format":"png"}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out))
	}
	if got := gjson.GetBytes(out[0], "candidates.0.content.parts.0.inlineData.data").String(); got != "YWJj" {
		t.Fatalf("unexpected inline data: %q", got)
	}
	if got := gjson.GetBytes(out[0], "candidates.0.content.parts.0.inlineData.mimeType").String(); got != "image/png" {
		t.Fatalf("unexpected mime type: %q", got)
	}

	out = ConvertCodexResponseToGemini(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.image_generation_call.partial_image","item_id":"img_1","partial_image_b64":"YWJj","output_format":"png"}`), &param)
	if len(out) != 0 {
		t.Fatalf("expected duplicate partial image to be ignored, got %d chunks", len(out))
	}
}

func TestConvertCodexResponseToGemini_NonStreamIncludesGeneratedImages(t *testing.T) {
	ctx := context.Background()
	out := ConvertCodexResponseToGeminiNonStream(ctx, "gpt-5.4", nil, nil, []byte(`{"type":"response.completed","response":{"id":"resp_123","created_at":1700000000,"output":[{"type":"image_generation_call","result":"YWJj","output_format":"png"}]}}`), nil)

	if got := gjson.GetBytes(out, "candidates.0.content.parts.0.inlineData.data").String(); got != "YWJj" {
		t.Fatalf("unexpected inline data: %q body=%s", got, string(out))
	}
	if got := gjson.GetBytes(out, "candidates.0.content.parts.0.inlineData.mimeType").String(); got != "image/png" {
		t.Fatalf("unexpected mime type: %q body=%s", got, string(out))
	}
}
