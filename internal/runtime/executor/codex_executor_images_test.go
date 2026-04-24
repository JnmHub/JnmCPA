package executor

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestEnsureImageGenerationToolAddsToolWhenMissing(t *testing.T) {
	body := []byte(`{"tools":[{"type":"function","name":"web_search"}]}`)
	out := ensureImageGenerationTool(body, "gpt-5.4")

	if got := gjson.GetBytes(out, "tools.#(type==\"image_generation\").type").String(); got != "image_generation" {
		t.Fatalf("image_generation tool not added: %s", string(out))
	}
}

func TestEnsureImageGenerationToolDoesNotDuplicateExistingTool(t *testing.T) {
	body := []byte(`{"tools":[{"type":"image_generation","output_format":"png"}]}`)
	out := ensureImageGenerationTool(body, "gpt-5.4")

	if got := len(gjson.GetBytes(out, "tools").Array()); got != 1 {
		t.Fatalf("unexpected tool count: %d body=%s", got, string(out))
	}
}

func TestEnsureImageGenerationToolSkipsSparkModels(t *testing.T) {
	body := []byte(`{"tools":[{"type":"function","name":"web_search"}]}`)
	out := ensureImageGenerationTool(body, "gpt-5.3-codex-spark")

	if got := gjson.GetBytes(out, "tools.#(type==\"image_generation\").type").String(); got != "" {
		t.Fatalf("unexpected image_generation tool for spark model: %s", string(out))
	}
}
