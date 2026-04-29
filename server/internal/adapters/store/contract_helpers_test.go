package store

import (
	"reflect"
	"testing"
	"time"
)

func contractTime(offset int) time.Time {
	return time.Date(2026, time.April, 29, 12, 0, offset, 0, time.UTC)
}

func assertDeepEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	gotValue := reflect.ValueOf(got)
	wantValue := reflect.ValueOf(want)
	if gotValue.IsValid() && wantValue.IsValid() &&
		gotValue.Kind() == reflect.Slice && wantValue.Kind() == reflect.Slice &&
		gotValue.Len() == 0 && wantValue.Len() == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s mismatch\n got: %#v\nwant: %#v", label, got, want)
	}
}
