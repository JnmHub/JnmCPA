package executor

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type codexNonStreamOutputState struct {
	key         string
	itemType    string
	itemID      string
	outputIndex int
	firstSeq    int
	rawItem     []byte

	callID           string
	name             string
	encryptedContent string
	outputFormat     string
	resultB64        string

	textDone      string
	textBuilder   strings.Builder
	reasoningDone string
	reasoningBuf  strings.Builder
	argsDone      string
	argsBuilder   strings.Builder
}

type codexNonStreamAggregator struct {
	responseRaw       []byte
	completedEventRaw []byte
	responseID        string
	model             string
	createdAt         int64
	usageRaw          []byte
	seenCompleted     bool
	seq               int
	items             map[string]*codexNonStreamOutputState
}

func aggregateCodexNonStreamCompletion(events [][]byte) ([]byte, bool) {
	if len(events) == 0 {
		return nil, false
	}

	agg := &codexNonStreamAggregator{
		items: make(map[string]*codexNonStreamOutputState),
	}
	for _, event := range events {
		agg.ingest(event)
	}
	return agg.build()
}

func (a *codexNonStreamAggregator) ingest(payload []byte) {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) || !gjson.ValidBytes(payload) {
		return
	}

	root := gjson.ParseBytes(payload)
	eventType := strings.TrimSpace(root.Get("type").String())
	if eventType == "" {
		return
	}

	a.seq++

	if responseNode := root.Get("response"); responseNode.Exists() {
		a.responseRaw = append(a.responseRaw[:0], responseNode.Raw...)
		if v := responseNode.Get("id"); v.Exists() && strings.TrimSpace(v.String()) != "" {
			a.responseID = v.String()
		}
		if v := responseNode.Get("model"); v.Exists() && strings.TrimSpace(v.String()) != "" {
			a.model = v.String()
		}
		if v := responseNode.Get("created_at"); v.Exists() && v.Int() > 0 {
			a.createdAt = v.Int()
		}
		if v := responseNode.Get("usage"); v.Exists() {
			a.usageRaw = append(a.usageRaw[:0], v.Raw...)
		}
		if eventType == "response.completed" {
			a.seenCompleted = true
			a.completedEventRaw = append(a.completedEventRaw[:0], payload...)
		}
	}

	switch eventType {
	case "response.output_item.added", "response.output_item.done":
		item := root.Get("item")
		if !item.Exists() {
			return
		}
		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType == "" {
			return
		}
		state := a.itemState(itemType, item.Get("id").String(), outputIndexFromResult(root))
		state.mergeItem(item)
	case "response.output_text.delta":
		state := a.itemState("message", root.Get("item_id").String(), outputIndexFromResult(root))
		state.textBuilder.WriteString(root.Get("delta").String())
	case "response.output_text.done":
		state := a.itemState("message", root.Get("item_id").String(), outputIndexFromResult(root))
		if text := root.Get("text").String(); text != "" {
			state.textDone = text
		}
	case "response.reasoning_summary_text.delta":
		state := a.itemState("reasoning", root.Get("item_id").String(), outputIndexFromResult(root))
		state.reasoningBuf.WriteString(root.Get("delta").String())
	case "response.reasoning_summary_text.done":
		state := a.itemState("reasoning", root.Get("item_id").String(), outputIndexFromResult(root))
		if text := root.Get("text").String(); text != "" {
			state.reasoningDone = text
		}
	case "response.function_call_arguments.delta":
		state := a.itemState("function_call", root.Get("item_id").String(), outputIndexFromResult(root))
		state.argsBuilder.WriteString(root.Get("delta").String())
	case "response.function_call_arguments.done":
		state := a.itemState("function_call", root.Get("item_id").String(), outputIndexFromResult(root))
		if arguments := root.Get("arguments").String(); arguments != "" {
			state.argsDone = arguments
		}
	case "response.image_generation_call.partial_image":
		state := a.itemState("image_generation_call", root.Get("item_id").String(), outputIndexFromResult(root))
		if b64 := root.Get("partial_image_b64").String(); b64 != "" {
			state.resultB64 = b64
		}
		if outputFormat := root.Get("output_format").String(); outputFormat != "" {
			state.outputFormat = outputFormat
		}
	}
}

func (a *codexNonStreamAggregator) itemState(itemType, itemID string, outputIndex int) *codexNonStreamOutputState {
	key := codexOutputStateKey(itemType, itemID, outputIndex)
	if state, ok := a.items[key]; ok {
		if state.itemID == "" && strings.TrimSpace(itemID) != "" {
			state.itemID = itemID
		}
		if state.outputIndex < 0 && outputIndex >= 0 {
			state.outputIndex = outputIndex
		}
		return state
	}
	state := &codexNonStreamOutputState{
		key:         key,
		itemType:    itemType,
		itemID:      strings.TrimSpace(itemID),
		outputIndex: outputIndex,
		firstSeq:    a.seq,
	}
	a.items[key] = state
	return state
}

func codexOutputStateKey(itemType, itemID string, outputIndex int) string {
	itemType = strings.TrimSpace(itemType)
	itemID = strings.TrimSpace(itemID)
	switch {
	case itemID != "":
		return itemType + ":" + itemID
	case outputIndex >= 0:
		return fmt.Sprintf("%s#%d", itemType, outputIndex)
	default:
		return itemType
	}
}

func outputIndexFromResult(result gjson.Result) int {
	v := result.Get("output_index")
	if !v.Exists() {
		return -1
	}
	return int(v.Int())
}

func (s *codexNonStreamOutputState) mergeItem(item gjson.Result) {
	if !item.Exists() {
		return
	}

	raw := []byte(item.Raw)
	if len(raw) > 0 {
		shouldReplace := len(s.rawItem) == 0
		if !shouldReplace {
			currentStatus := gjson.GetBytes(s.rawItem, "status").String()
			nextStatus := item.Get("status").String()
			if currentStatus != "completed" && nextStatus == "completed" {
				shouldReplace = true
			}
		}
		if shouldReplace {
			s.rawItem = append(s.rawItem[:0], raw...)
		}
	}

	if id := strings.TrimSpace(item.Get("id").String()); id != "" {
		s.itemID = id
	}
	if callID := strings.TrimSpace(item.Get("call_id").String()); callID != "" {
		s.callID = callID
	}
	if name := strings.TrimSpace(item.Get("name").String()); name != "" {
		s.name = name
	}
	if enc := strings.TrimSpace(item.Get("encrypted_content").String()); enc != "" {
		s.encryptedContent = enc
	}
	if outputFormat := strings.TrimSpace(item.Get("output_format").String()); outputFormat != "" {
		s.outputFormat = outputFormat
	}
	if resultB64 := strings.TrimSpace(item.Get("result").String()); resultB64 != "" {
		s.resultB64 = resultB64
	}
	if args := item.Get("arguments").String(); args != "" {
		s.argsDone = args
	}

	if text := extractMessageText(item); text != "" {
		s.textDone = text
	}
	if reasoning := extractReasoningSummaryText(item); reasoning != "" {
		s.reasoningDone = reasoning
	}
}

func (a *codexNonStreamAggregator) build() ([]byte, bool) {
	if !a.seenCompleted {
		return nil, false
	}

	event := []byte(`{"type":"response.completed","response":{"object":"response","status":"completed","output":[]}}`)
	if len(a.completedEventRaw) > 0 && gjson.ValidBytes(a.completedEventRaw) {
		event = append([]byte(nil), a.completedEventRaw...)
	} else if len(a.responseRaw) > 0 && gjson.ValidBytes(a.responseRaw) {
		event, _ = sjson.SetRawBytes(event, "response", a.responseRaw)
	}

	if !gjson.GetBytes(event, "response").Exists() {
		event, _ = sjson.SetRawBytes(event, "response", []byte(`{"object":"response","status":"completed","output":[]}`))
	}
	event, _ = sjson.SetBytes(event, "type", "response.completed")
	event, _ = sjson.SetBytes(event, "response.object", "response")
	if !gjson.GetBytes(event, "response.status").Exists() || strings.TrimSpace(gjson.GetBytes(event, "response.status").String()) == "" {
		event, _ = sjson.SetBytes(event, "response.status", "completed")
	}
	if a.responseID != "" && strings.TrimSpace(gjson.GetBytes(event, "response.id").String()) == "" {
		event, _ = sjson.SetBytes(event, "response.id", a.responseID)
	}
	if a.model != "" && strings.TrimSpace(gjson.GetBytes(event, "response.model").String()) == "" {
		event, _ = sjson.SetBytes(event, "response.model", a.model)
	}
	if a.createdAt > 0 && gjson.GetBytes(event, "response.created_at").Int() == 0 {
		event, _ = sjson.SetBytes(event, "response.created_at", a.createdAt)
	}
	if len(a.usageRaw) > 0 && gjson.ValidBytes(a.usageRaw) {
		event, _ = sjson.SetRawBytes(event, "response.usage", a.usageRaw)
	}

	aggregated := a.buildAggregatedOutputs()
	merged := mergeCodexAggregatedOutputs(aggregated, gjson.GetBytes(event, "response.output").Array())
	if len(merged) > 0 {
		wrapper := []byte(`{"arr":[]}`)
		for _, item := range merged {
			wrapper, _ = sjson.SetRawBytes(wrapper, "arr.-1", item)
		}
		event, _ = sjson.SetRawBytes(event, "response.output", []byte(gjson.GetBytes(wrapper, "arr").Raw))
	} else if !gjson.GetBytes(event, "response.output").Exists() {
		event, _ = sjson.SetRawBytes(event, "response.output", []byte(`[]`))
	}

	return event, true
}

func (a *codexNonStreamAggregator) buildAggregatedOutputs() [][]byte {
	states := make([]*codexNonStreamOutputState, 0, len(a.items))
	for _, state := range a.items {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		left := states[i]
		right := states[j]
		leftIndex := left.outputIndex
		rightIndex := right.outputIndex
		switch {
		case leftIndex < 0 && rightIndex >= 0:
			return false
		case leftIndex >= 0 && rightIndex < 0:
			return true
		case leftIndex != rightIndex:
			return leftIndex < rightIndex
		default:
			return left.firstSeq < right.firstSeq
		}
	})

	out := make([][]byte, 0, len(states))
	for _, state := range states {
		if item := state.buildItem(); len(item) > 0 {
			out = append(out, item)
		}
	}
	return out
}

func mergeCodexAggregatedOutputs(aggregated [][]byte, original []gjson.Result) [][]byte {
	if len(original) == 0 {
		return aggregated
	}

	merged := append([][]byte(nil), aggregated...)
	seen := make(map[string]struct{}, len(aggregated))
	for i, item := range aggregated {
		seen[dedupeCodexOutputKey(gjson.ParseBytes(item), i)] = struct{}{}
	}
	for i, item := range original {
		key := dedupeCodexOutputKey(item, i)
		if _, ok := seen[key]; ok {
			continue
		}
		merged = append(merged, []byte(item.Raw))
	}
	return merged
}

func dedupeCodexOutputKey(item gjson.Result, index int) string {
	if id := strings.TrimSpace(item.Get("id").String()); id != "" {
		return "id:" + id
	}
	if callID := strings.TrimSpace(item.Get("call_id").String()); callID != "" {
		return "call:" + callID
	}
	return fmt.Sprintf("%s#%d", strings.TrimSpace(item.Get("type").String()), index)
}

func (s *codexNonStreamOutputState) buildItem() []byte {
	switch s.itemType {
	case "message":
		return s.buildMessageItem()
	case "reasoning":
		return s.buildReasoningItem()
	case "function_call":
		return s.buildFunctionCallItem()
	case "image_generation_call":
		return s.buildImageGenerationItem()
	default:
		if len(s.rawItem) > 0 {
			return append([]byte(nil), s.rawItem...)
		}
		return nil
	}
}

func (s *codexNonStreamOutputState) buildMessageItem() []byte {
	item := defaultIfEmptyRaw(s.rawItem, []byte(`{"id":"","type":"message","status":"completed","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":""}],"role":"assistant"}`))
	text := firstNonEmptyString(s.textDone, s.textBuilder.String(), extractMessageText(gjson.ParseBytes(item)))
	if text == "" && len(s.rawItem) == 0 {
		return nil
	}
	item, _ = sjson.SetBytes(item, "type", "message")
	item, _ = sjson.SetBytes(item, "status", "completed")
	item, _ = sjson.SetBytes(item, "role", firstNonEmptyString(gjson.GetBytes(item, "role").String(), "assistant"))
	if s.itemID != "" {
		item, _ = sjson.SetBytes(item, "id", s.itemID)
	}
	if !gjson.GetBytes(item, "content").Exists() {
		item, _ = sjson.SetRawBytes(item, "content", []byte(`[{"type":"output_text","annotations":[],"logprobs":[],"text":""}]`))
	}
	if !gjson.GetBytes(item, "content.0.type").Exists() {
		item, _ = sjson.SetBytes(item, "content.0.type", "output_text")
	}
	if text != "" {
		item, _ = sjson.SetBytes(item, "content.0.text", text)
	}
	return item
}

func (s *codexNonStreamOutputState) buildReasoningItem() []byte {
	item := defaultIfEmptyRaw(s.rawItem, []byte(`{"id":"","type":"reasoning","status":"completed","summary":[]}`))
	text := firstNonEmptyString(s.reasoningDone, s.reasoningBuf.String(), extractReasoningSummaryText(gjson.ParseBytes(item)))
	if text == "" && s.encryptedContent == "" && len(s.rawItem) == 0 {
		return nil
	}
	item, _ = sjson.SetBytes(item, "type", "reasoning")
	item, _ = sjson.SetBytes(item, "status", "completed")
	if s.itemID != "" {
		item, _ = sjson.SetBytes(item, "id", s.itemID)
	}
	if s.encryptedContent != "" {
		item, _ = sjson.SetBytes(item, "encrypted_content", s.encryptedContent)
	}
	if text != "" {
		item, _ = sjson.SetRawBytes(item, "summary", []byte(`[{"type":"summary_text","text":""}]`))
		item, _ = sjson.SetBytes(item, "summary.0.type", "summary_text")
		item, _ = sjson.SetBytes(item, "summary.0.text", text)
	}
	return item
}

func (s *codexNonStreamOutputState) buildFunctionCallItem() []byte {
	item := defaultIfEmptyRaw(s.rawItem, []byte(`{"id":"","type":"function_call","status":"completed","arguments":"","call_id":"","name":""}`))
	callID := firstNonEmptyString(s.callID, gjson.GetBytes(item, "call_id").String())
	if callID == "" && strings.HasPrefix(s.itemID, "fc_") {
		callID = strings.TrimPrefix(s.itemID, "fc_")
	}
	name := firstNonEmptyString(s.name, gjson.GetBytes(item, "name").String())
	args := firstNonEmptyString(s.argsDone, s.argsBuilder.String(), gjson.GetBytes(item, "arguments").String())
	if callID == "" && name == "" && args == "" && len(s.rawItem) == 0 {
		return nil
	}
	item, _ = sjson.SetBytes(item, "type", "function_call")
	item, _ = sjson.SetBytes(item, "status", "completed")
	if s.itemID != "" {
		item, _ = sjson.SetBytes(item, "id", s.itemID)
	} else if callID != "" {
		item, _ = sjson.SetBytes(item, "id", "fc_"+callID)
	}
	if callID != "" {
		item, _ = sjson.SetBytes(item, "call_id", callID)
	}
	if name != "" {
		item, _ = sjson.SetBytes(item, "name", name)
	}
	if args != "" {
		item, _ = sjson.SetBytes(item, "arguments", args)
	}
	return item
}

func (s *codexNonStreamOutputState) buildImageGenerationItem() []byte {
	item := defaultIfEmptyRaw(s.rawItem, []byte(`{"id":"","type":"image_generation_call","status":"completed","result":"","output_format":""}`))
	resultB64 := firstNonEmptyString(s.resultB64, gjson.GetBytes(item, "result").String())
	if resultB64 == "" {
		return nil
	}
	outputFormat := firstNonEmptyString(s.outputFormat, gjson.GetBytes(item, "output_format").String())
	item, _ = sjson.SetBytes(item, "type", "image_generation_call")
	item, _ = sjson.SetBytes(item, "status", "completed")
	if s.itemID != "" {
		item, _ = sjson.SetBytes(item, "id", s.itemID)
	}
	item, _ = sjson.SetBytes(item, "result", resultB64)
	if outputFormat != "" {
		item, _ = sjson.SetBytes(item, "output_format", outputFormat)
	}
	return item
}

func defaultIfEmptyRaw(raw []byte, fallback []byte) []byte {
	if len(raw) > 0 && gjson.ValidBytes(raw) {
		return append([]byte(nil), raw...)
	}
	return append([]byte(nil), fallback...)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func extractMessageText(item gjson.Result) string {
	content := item.Get("content")
	if !content.Exists() || !content.IsArray() {
		return ""
	}
	var builder strings.Builder
	content.ForEach(func(_, part gjson.Result) bool {
		if part.Get("type").String() == "output_text" {
			builder.WriteString(part.Get("text").String())
		}
		return true
	})
	return builder.String()
}

func extractReasoningSummaryText(item gjson.Result) string {
	summary := item.Get("summary")
	if summary.Exists() && summary.IsArray() {
		var builder strings.Builder
		summary.ForEach(func(_, part gjson.Result) bool {
			if text := part.Get("text").String(); text != "" {
				builder.WriteString(text)
			}
			return true
		})
		if builder.Len() > 0 {
			return builder.String()
		}
	}
	return ""
}
