package reflect_util

import (
	"reflect"
	"testing"
)

type testStruct struct {
	Name  string
	Age   int
	Email string
}

func TestGetFields(t *testing.T) {
	fields := GetFields(reflect.TypeOf(testStruct{}))
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	expected := []string{"Name", "Age", "Email"}
	for i, f := range fields {
		if f.Name != expected[i] {
			t.Errorf("field[%d] = %q, want %q", i, f.Name, expected[i])
		}
	}
}

func TestGetFields_Empty(t *testing.T) {
	fields := GetFields(reflect.TypeOf(struct{}{}))
	if len(fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(fields))
	}
}

func TestGetFieldValues(t *testing.T) {
	s := testStruct{Name: "Alice", Age: 30, Email: "alice@test.com"}
	vals := GetFieldValues(reflect.ValueOf(s))
	if len(vals) != 3 {
		t.Fatalf("expected 3 values, got %d", len(vals))
	}
	if vals[0].String() != "Alice" {
		t.Errorf("val[0] = %q, want 'Alice'", vals[0].String())
	}
	if vals[1].Int() != 30 {
		t.Errorf("val[1] = %d, want 30", vals[1].Int())
	}
}

func TestGetFieldValues_Empty(t *testing.T) {
	vals := GetFieldValues(reflect.ValueOf(struct{}{}))
	if len(vals) != 0 {
		t.Errorf("expected 0 values, got %d", len(vals))
	}
}

func TestGetFields_Nil(t *testing.T) {
	// Calling GetFields on a nil interface should panic — verify recovery works.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil type")
		}
	}()
	GetFields(nil)
}
