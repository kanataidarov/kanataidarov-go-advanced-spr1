package models

import (
	"reflect"
	"testing"

	"github.com/mailru/easyjson"
)

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int64) *int64       { return &v }

func TestMetricsMarshalEasyJSON(t *testing.T) {
	tests := []struct {
		name   string
		metric Metrics
		want   string
	}{
		{
			name:   "gauge",
			metric: Metrics{ID: "Alloc", MType: Gauge, Value: ptrFloat(12.5)},
			want:   `{"id":"Alloc","type":"gauge","value":12.5}`,
		},
		{
			name:   "counter",
			metric: Metrics{ID: "PollCount", MType: Counter, Delta: ptrInt(5)},
			want:   `{"id":"PollCount","type":"counter","delta":5}`,
		},
		{
			name:   "zero values are kept",
			metric: Metrics{ID: "Zero", MType: Counter, Delta: ptrInt(0)},
			want:   `{"id":"Zero","type":"counter","delta":0}`,
		},
		{
			name:   "empty optional fields are omitted",
			metric: Metrics{ID: "LastGC", MType: Gauge},
			want:   `{"id":"LastGC","type":"gauge"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := easyjson.Marshal(tt.metric)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMetricsUnmarshalEasyJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want Metrics
	}{
		{
			name: "gauge",
			data: `{"id":"LastGC","type":"gauge","value":1744184459}`,
			want: Metrics{ID: "LastGC", MType: Gauge, Value: ptrFloat(1744184459)},
		},
		{
			name: "counter",
			data: `{"id":"PollCount","type":"counter","delta":7}`,
			want: Metrics{ID: "PollCount", MType: Counter, Delta: ptrInt(7)},
		},
		{
			name: "request for a value",
			data: `{"id":"LastGC","type":"gauge"}`,
			want: Metrics{ID: "LastGC", MType: Gauge},
		},
		{
			name: "nulls and unknown fields",
			data: `{"id":"X","type":"gauge","value":null,"delta":null,"extra":{"a":[1,2]}}`,
			want: Metrics{ID: "X", MType: Gauge},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Metrics
			if err := easyjson.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMetricsUnmarshalEasyJSONRejectsInvalidInput(t *testing.T) {
	cases := []string{
		``,
		`not json`,
		`{"id":"a"`,
		`{"id":1,"type":"gauge"}`,
		`{"id":"a","type":"counter","delta":"5"}`,
		`{"id":"a","type":"counter","delta":1.5}`,
		`{"id":"a","type":"gauge","value":"abc"}`,
	}

	for _, data := range cases {
		var m Metrics
		if err := easyjson.Unmarshal([]byte(data), &m); err == nil {
			t.Errorf("%q must produce an error", data)
		}
	}
}

func TestMetricsListEasyJSON(t *testing.T) {
	list := MetricsList{
		{ID: "Alloc", MType: Gauge, Value: ptrFloat(1.5)},
		{ID: "PollCount", MType: Counter, Delta: ptrInt(3)},
	}

	data, err := easyjson.Marshal(list)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `[{"id":"Alloc","type":"gauge","value":1.5},{"id":"PollCount","type":"counter","delta":3}]`
	if string(data) != want {
		t.Errorf("got %s, want %s", data, want)
	}

	var decoded MetricsList
	if err := easyjson.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(decoded, list) {
		t.Errorf("got %+v, want %+v", decoded, list)
	}

	if err := easyjson.Unmarshal([]byte(`{"id":"Alloc"}`), &decoded); err == nil {
		t.Error("an object must not decode as a list")
	}
}
