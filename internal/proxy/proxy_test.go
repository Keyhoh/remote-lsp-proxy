package proxy

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeAndDecodeJSONRPCMessages(t *testing.T) {
	msg := `{"jsonrpc":"2.0","method":"initialize","params":{"processId":null}}`
	buf := EncodeJSONRPCMessage([]byte(msg))

	decoded, err := DecodeJSONRPCMessages(bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("DecodeJSONRPCMessages returned error: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 message, got %d", len(decoded))
	}
	if string(decoded[0]) != msg {
		t.Fatalf("decoded payload mismatch: got %s want %s", string(decoded[0]), msg)
	}
}

func TestDecodeJSONRPCMessagesHandlesSplitAndBatch(t *testing.T) {
	msg1 := `{"jsonrpc":"2.0","method":"initialized"}`
	msg2 := `{"jsonrpc":"2.0","method":"exit"}`
	payload := append(EncodeJSONRPCMessage([]byte(msg1)), EncodeJSONRPCMessage([]byte(msg2))...)

	decoded, err := DecodeJSONRPCMessages(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("DecodeJSONRPCMessages returned error: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(decoded))
	}
	if string(decoded[0]) != msg1 || string(decoded[1]) != msg2 {
		t.Fatalf("unexpected batched messages: %q %q", string(decoded[0]), string(decoded[1]))
	}
}

func TestTranslateFileURIs(t *testing.T) {
	p := New(Config{LocalWorkspace: "/home/user/project", RemoteWorkspace: "/workspace/project"})

	if got, want := p.translateURI("file:///home/user/project/src/main.go", LocalToRemote), "file:///workspace/project/src/main.go"; got != want {
		t.Fatalf("local->remote translation mismatch: got %s want %s", got, want)
	}
	if got, want := p.translateURI("file:///workspace/project/src/main.go", RemoteToLocal), "file:///home/user/project/src/main.go"; got != want {
		t.Fatalf("remote->local translation mismatch: got %s want %s", got, want)
	}
	if got, want := p.translateURI("https://example.com/docs", LocalToRemote), "https://example.com/docs"; got != want {
		t.Fatalf("non-file scheme should be unchanged: got %s want %s", got, want)
	}
}

func TestTransformInitializeMessage(t *testing.T) {
	p := New(Config{LocalWorkspace: "/home/user/project", RemoteWorkspace: "/workspace/project"})
	payload := []byte(`{"jsonrpc":"2.0","method":"initialize","params":{"processId":123,"rootUri":"file:///home/user/project","workspaceFolders":[{"uri":"file:///home/user/project","name":"project"}]}}`)

	transformed, err := p.transformJSONRPC(payload, LocalToRemote)
	if err != nil {
		t.Fatalf("transformJSONRPC returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(transformed, &decoded); err != nil {
		t.Fatalf("unmarshal transformed message: %v", err)
	}
	params := decoded["params"].(map[string]any)
	if params["processId"] != nil {
		t.Fatalf("processId should be normalized to nil, got %#v", params["processId"])
	}
	if got, want := params["rootUri"], "file:///workspace/project"; got != want {
		t.Fatalf("rootUri mismatch: got %#v want %#v", got, want)
	}
	folders := params["workspaceFolders"].([]any)
	folder := folders[0].(map[string]any)
	if got, want := folder["uri"], "file:///workspace/project"; got != want {
		t.Fatalf("workspaceFolders uri mismatch: got %#v want %#v", got, want)
	}
}
