package catalogs

import (
	"reflect"
	"testing"
)

func TestOptionalModelRecordAPI(t *testing.T) {
	fields := reflect.TypeFor[Model]()
	pointers := make(map[string]int)
	for index := range fields.NumField() {
		field := fields.Field(index)
		if field.IsExported() && field.Type.Kind() == reflect.Pointer {
			name := field.Tag.Get("json")
			for i, r := range name {
				if r == ',' {
					name = name[:i]
					break
				}
			}
			pointers[name] = index
		}
	}
	records := PublishedModelRecords()
	if len(records) != len(pointers) {
		t.Fatalf("record descriptor count=%d, pointer fields=%d", len(records), len(pointers))
	}
	for _, record := range records {
		t.Run(string(record), func(t *testing.T) {
			index, exists := pointers[string(record)]
			if !exists {
				t.Fatalf("unsupported descriptor %q", record)
			}
			var model Model
			if model.RecordPresence(record) != ValueMissing {
				t.Fatal("zero record is not missing")
			}
			if !model.SetRecordUnknown(record) || model.RecordPresence(record) != ValueUnknown {
				t.Fatal("unknown claim did not persist")
			}
			for _, other := range records {
				if other != record && model.RecordPresence(other) != ValueMissing {
					t.Fatalf("claim affected unrelated record %q", other)
				}
			}
			field := reflect.ValueOf(&model).Elem().Field(index)
			field.Set(reflect.New(field.Type().Elem()))
			if model.RecordPresence(record) != ValueKnown {
				t.Fatal("explicit record did not replace the unknown value")
			}
			if !model.UnsetRecord(record) || !field.IsNil() || model.RecordPresence(record) != ValueMissing {
				t.Fatal("unset left a value or presence claim")
			}
		})
	}
	records[0] = "caller-change"
	if PublishedModelRecords()[0] == "caller-change" {
		t.Fatal("caller changed the record descriptors")
	}
	var nilModel *Model
	for _, model := range []*Model{nilModel, {}} {
		if model.SetRecordUnknown("id") || model.UnsetRecord("id") || model.RecordPresence("id") != ValueMissing {
			t.Fatal("invalid optional record changed required identity")
		}
	}
}
