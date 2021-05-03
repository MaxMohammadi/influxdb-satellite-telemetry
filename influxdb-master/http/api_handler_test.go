package http

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	kithttp "github.com/influxdata/influxdb/v2/kit/transport/http"
	"github.com/influxdata/influxdb/v2/pkg/httpc"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
	"go.uber.org/zap/zaptest"
)

func TestAPIHandler_NotFound(t *testing.T) {
	type args struct {
		method string
		path   string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "path not found",
			args: args{
				method: "GET",
				path:   "/404",
			},
			wants: wants{
				statusCode:  http.StatusNotFound,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "code": "not found",
  "message": "path not found"
}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			r := httptest.NewRequest(tt.args.method, tt.args.path, nil)
			w := httptest.NewRecorder()

			b := &APIBackend{
				HTTPErrorHandler: kithttp.ErrorHandler(0),
				Logger:           zaptest.NewLogger(t),
			}

			h := NewAPIHandler(b)
			h.ServeHTTP(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. get %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. get %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
				t.Errorf("%q, error unmarshalling json %v", tt.name, err)
			} else if tt.wants.body != "" && !eq {
				t.Errorf("%q. ***%s***", tt.name, diff)
			}
		})
	}
}

func jsonEqual(s1, s2 string) (eq bool, diff string, err error) {
	if s1 == s2 {
		return true, "", nil
	}

	if s1 == "" {
		return false, s2, fmt.Errorf("s1 is empty")
	}

	if s2 == "" {
		return false, s1, fmt.Errorf("s2 is empty")
	}

	var o1 interface{}
	if err = json.Unmarshal([]byte(s1), &o1); err != nil {
		return
	}

	var o2 interface{}
	if err = json.Unmarshal([]byte(s2), &o2); err != nil {
		return
	}

	differ := gojsondiff.New()
	d, err := differ.Compare([]byte(s1), []byte(s2))
	if err != nil {
		return
	}

	config := formatter.AsciiFormatterConfig{}

	formatter := formatter.NewAsciiFormatter(o1, config)
	diff, err = formatter.Format(d)

	return cmp.Equal(o1, o2), diff, err
}

func mustNewHTTPClient(t *testing.T, addr, token string) *httpc.Client {
	t.Helper()

	httpClient, err := NewHTTPClient(addr, token, false)
	if err != nil {
		t.Fatal(err)
	}
	return httpClient
}
