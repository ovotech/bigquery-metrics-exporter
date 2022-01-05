package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceStatus_Handler(t *testing.T) {
	type fields struct {
		Status Status
	}
	type want struct {
		status int
		body   string
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{"health ok", fields{Status: Ok}, want{200, "{\"status\":\"OK\"}"}},
		{"health fail", fields{Status: Error}, want{500, "{\"status\":\"Error\"}"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := ServiceStatus{
				Status: tt.fields.Status,
			}

			req, err := http.NewRequest("GET", "/health", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(hs.Handler)

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.want.status {
				t.Errorf("handler returned wrong status code: got %v want %v",
					rr.Code, tt.want.status)
			}

			if rr.Body.String() != tt.want.body {
				t.Errorf("handler returned unexpected body: got %v want %v",
					rr.Body.String(), tt.want.body)
			}
		})
	}
}
