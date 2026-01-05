package icon

import "testing"

func TestIconData(t *testing.T) {
	if Data == nil {
		t.Error("Icon Data is nil")
	}
	if len(Data) == 0 {
		t.Error("Icon Data is empty")
	}
}
