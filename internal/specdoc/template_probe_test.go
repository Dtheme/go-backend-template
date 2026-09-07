package specdoc

import (
	"os"
	"testing"
)

func TestRealTemplateParses(t *testing.T) {
	doc, err := os.ReadFile("../../Specs/technical/技术方案模版.md")
	if err != nil {
		t.Fatal(err)
	}
	ds, err := ParseDeliveryStatus(doc)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Stage != "planning" || ds.UserAcceptance != "pending" || ds.Review != "not_required" {
		t.Fatalf("template block = %+v", ds)
	}
}
