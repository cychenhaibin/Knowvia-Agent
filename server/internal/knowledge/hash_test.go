package knowledge

import (
	"strings"
	"testing"
)

func TestNormalizeBody(t *testing.T) {
	body := "<p>Hello&nbsp;World</p>\n<div>Line 2</div>"
	got := NormalizeBody(body)
	if got != "Hello World\nLine 2" {
		t.Fatalf("unexpected normalized body: %q", got)
	}
}

func TestNormalizeBodyLakeTable(t *testing.T) {
	body := `{
		"format":"laketable",
		"sheet":[{
			"columns":[
				{"id":"stage","name":"阶段","options":[
					{"id":"resume","value":"简历筛选"},
					{"id":"interview","value":"一面"}
				]},
				{"id":"owner","name":"负责人"}
			],
			"views":{
				"view1":{
					"groupData":[
						{"groupBy":"stage","titleValue":["resume"],"rows":[{"id":"1"},{"id":"2"}]},
						{"groupBy":"stage","titleValue":["interview"],"rows":[{"id":"3"}]}
					]
				}
			}
		}]
	}`
	got := NormalizeBody(body)
	wantParts := []string{
		"表格列: 阶段, 负责人",
		"字段选项:",
		"- 阶段: 简历筛选, 一面",
		"分组统计:",
		"- 一面: 1 条",
		"- 简历筛选: 2 条",
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("normalized laketable missing %q in %q", want, got)
		}
	}
}

func TestSplitIntoChunks(t *testing.T) {
	chunks := SplitIntoChunks("alpha\nbeta\ngamma", 10)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
}
