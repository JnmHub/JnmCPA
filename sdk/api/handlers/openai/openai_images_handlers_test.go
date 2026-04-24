package openai

import (
	"context"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/tidwall/gjson"
)

func TestBuildImagesResponsesRequestIncludesPromptImagesAndTool(t *testing.T) {
	tool := []byte(`{"type":"image_generation","action":"edit","model":"gpt-image-2"}`)
	req := buildImagesResponsesRequest("draw a cat", []string{"data:image/png;base64,AAA", "https://example.com/b.png"}, tool)

	if got := gjson.GetBytes(req, "model").String(); got != defaultImagesMainModel {
		t.Fatalf("model = %q, want %q", got, defaultImagesMainModel)
	}
	if got := gjson.GetBytes(req, "input.0.content.0.text").String(); got != "draw a cat" {
		t.Fatalf("prompt = %q", got)
	}
	if got := gjson.GetBytes(req, "input.0.content.1.image_url").String(); got != "data:image/png;base64,AAA" {
		t.Fatalf("first image = %q", got)
	}
	if got := gjson.GetBytes(req, "input.0.content.2.image_url").String(); got != "https://example.com/b.png" {
		t.Fatalf("second image = %q", got)
	}
	if got := gjson.GetBytes(req, "tools.0.type").String(); got != "image_generation" {
		t.Fatalf("tool type = %q", got)
	}
}

func TestCollectImagesFromResponsesStreamBuildsOpenAIImageResponse(t *testing.T) {
	data := make(chan []byte, 1)
	errs := make(chan *interfaces.ErrorMessage)
	close(errs)

	data <- []byte("data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1777000000,\"output\":[{\"type\":\"image_generation_call\",\"result\":\"YWJj\",\"revised_prompt\":\"cat\",\"output_format\":\"png\",\"size\":\"1024x1024\"}],\"tool_usage\":{\"image_gen\":{\"images\":1}}}}\n\n")
	close(data)

	out, errMsg := collectImagesFromResponsesStream(context.Background(), data, errs, "b64_json")
	if errMsg != nil {
		t.Fatalf("unexpected err: %+v", errMsg)
	}
	if got := gjson.GetBytes(out, "created").Int(); got != 1777000000 {
		t.Fatalf("created = %d", got)
	}
	if got := gjson.GetBytes(out, "data.0.b64_json").String(); got != "YWJj" {
		t.Fatalf("b64_json = %q", got)
	}
	if got := gjson.GetBytes(out, "data.0.revised_prompt").String(); got != "cat" {
		t.Fatalf("revised_prompt = %q", got)
	}
	if got := gjson.GetBytes(out, "usage.images").Int(); got != 1 {
		t.Fatalf("usage.images = %d", got)
	}
}

func TestCollectImagesFromResponsesStreamReturnsUpstreamError(t *testing.T) {
	data := make(chan []byte)
	errs := make(chan *interfaces.ErrorMessage, 1)
	errs <- &interfaces.ErrorMessage{StatusCode: http.StatusBadGateway}
	close(errs)

	_, errMsg := collectImagesFromResponsesStream(context.Background(), data, errs, "b64_json")
	if errMsg == nil {
		t.Fatal("expected error message")
	}
	if errMsg.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", errMsg.StatusCode)
	}
}
