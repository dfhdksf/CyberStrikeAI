package docgen

import "testing"

func TestErrResultIsError(t *testing.T) {
	r := errResult("boom %d", 1)
	if !r.IsError {
		t.Fatal("errResult 应 IsError=true")
	}
	if len(r.Content) != 1 || r.Content[0].Text != "boom 1" {
		t.Fatalf("unexpected content: %+v", r.Content)
	}
}

func TestTextResultNotError(t *testing.T) {
	r := textResult("hello")
	if r.IsError {
		t.Fatal("textResult 不应是错误")
	}
	if r.Content[0].Text != "hello" {
		t.Fatalf("unexpected: %+v", r.Content)
	}
}
