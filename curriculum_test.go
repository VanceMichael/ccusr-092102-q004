package curriculum_test

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCurriculumFixtureKeepsVersion(t *testing.T) {
	raw, err := os.ReadFile("fixtures/curriculum.json")
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value["curriculum_version"] == "" || value["competency_framework"] == "" {
		t.Fatal("课程与能力框架版本不能为空")
	}
}
