package libs

import "testing"

func TestNewPlugin(t *testing.T) {
	_, err := NewPlugin(".", "yd4go")
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewCPlugin(t *testing.T) {

}
