package worker

import "testing"

func TestDetectAskPattern(t *testing.T) {
	content := "some output\nASK:worker-2:How should I handle errors?\nmore output"
	targetID, question, found := detectAskPattern(content)
	if !found {
		t.Fatal("expected ASK pattern to be found")
	}
	if targetID != "worker-2" {
		t.Errorf("targetID = %q, want %q", targetID, "worker-2")
	}
	if question != "How should I handle errors?" {
		t.Errorf("question = %q, want %q", question, "How should I handle errors?")
	}
}

func TestDetectAskPattern_NoMatch(t *testing.T) {
	content := "normal output without ask\njust some lines\n"
	_, _, found := detectAskPattern(content)
	if found {
		t.Fatal("expected no ASK pattern to be found")
	}
}

func TestDetectAskPattern_MultiLine(t *testing.T) {
	content := "line 1\nASK:worker-1:First question?\nline 3\nASK:worker-3:Second question?\nline 5"
	targetID, question, found := detectAskPattern(content)
	if !found {
		t.Fatal("expected ASK pattern to be found")
	}
	// Should return the LAST occurrence (most recent)
	if targetID != "worker-3" {
		t.Errorf("targetID = %q, want %q (last occurrence)", targetID, "worker-3")
	}
	if question != "Second question?" {
		t.Errorf("question = %q, want %q", question, "Second question?")
	}
}

func TestDetectAskPattern_MalformedIgnored(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"only_prefix", "ASK:only-one-part"},
		{"empty_target", "ASK::some question"},
		{"empty_question", "ASK:worker-1:"},
		{"no_colon", "ASK"},
		{"single_colon", "ASK:something"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, found := detectAskPattern(tt.content)
			if found {
				t.Errorf("expected no match for %q", tt.content)
			}
		})
	}
}

func TestDetectReplyPattern(t *testing.T) {
	content := "some output\nREPLY:msg-123:Use fmt.Errorf for wrapping\nmore output"
	messageID, reply, found := detectReplyPattern(content)
	if !found {
		t.Fatal("expected REPLY pattern to be found")
	}
	if messageID != "msg-123" {
		t.Errorf("messageID = %q, want %q", messageID, "msg-123")
	}
	if reply != "Use fmt.Errorf for wrapping" {
		t.Errorf("reply = %q, want %q", reply, "Use fmt.Errorf for wrapping")
	}
}

func TestDetectReplyPattern_NoMatch(t *testing.T) {
	content := "normal output\nno reply here\n"
	_, _, found := detectReplyPattern(content)
	if found {
		t.Fatal("expected no REPLY pattern to be found")
	}
}

func TestDetectReplyPattern_WithColonsInContent(t *testing.T) {
	content := "REPLY:msg-456:Use http://example.com:8080 for the endpoint"
	messageID, reply, found := detectReplyPattern(content)
	if !found {
		t.Fatal("expected REPLY pattern to be found")
	}
	if messageID != "msg-456" {
		t.Errorf("messageID = %q, want %q", messageID, "msg-456")
	}
	// SplitN with n=3 should preserve colons in the third part
	expected := "Use http://example.com:8080 for the endpoint"
	if reply != expected {
		t.Errorf("reply = %q, want %q", reply, expected)
	}
}
