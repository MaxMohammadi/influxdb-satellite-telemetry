package http

import (
	"net/http/httptest"
	"testing"

	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/mock"
)

func TestPaging_DecodeFindOptions(t *testing.T) {
	type args struct {
		queryParams map[string]string
	}
	type wants struct {
		opts influxdb.FindOptions
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "decode FindOptions",
			args: args{
				map[string]string{
					"offset":     "10",
					"limit":      "10",
					"sortBy":     "updateTime",
					"descending": "true",
				},
			},
			wants: wants{
				opts: influxdb.FindOptions{
					Offset:     10,
					Limit:      10,
					SortBy:     "updateTime",
					Descending: true,
				},
			},
		},
		{
			name: "decode FindOptions with default values",
			args: args{
				map[string]string{
					"limit": "10",
				},
			},
			wants: wants{
				opts: influxdb.FindOptions{
					Offset:     0,
					Limit:      10,
					SortBy:     "",
					Descending: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://any.url", nil)
			qp := r.URL.Query()
			for k, v := range tt.args.queryParams {
				qp.Set(k, v)
			}
			r.URL.RawQuery = qp.Encode()

			opts, err := influxdb.DecodeFindOptions(r)
			if err != nil {
				t.Errorf("%q failed, err: %s", tt.name, err.Error())
			}

			if opts.Offset != tt.wants.opts.Offset {
				t.Errorf("%q. influxdb.DecodeFindOptions() = %v, want %v", tt.name, opts.Offset, tt.wants.opts.Offset)
			}
			if opts.Limit != tt.wants.opts.Limit {
				t.Errorf("%q. influxdb.DecodeFindOptions() = %v, want %v", tt.name, opts.Limit, tt.wants.opts.Limit)
			}
			if opts.SortBy != tt.wants.opts.SortBy {
				t.Errorf("%q. influxdb.DecodeFindOptions() = %v, want %v", tt.name, opts.SortBy, tt.wants.opts.SortBy)
			}
			if opts.Descending != tt.wants.opts.Descending {
				t.Errorf("%q. influxdb.DecodeFindOptions() = %v, want %v", tt.name, opts.Descending, tt.wants.opts.Descending)
			}
		})
	}
}

func TestPaging_NewPagingLinks(t *testing.T) {
	type args struct {
		basePath string
		num      int
		opts     influxdb.FindOptions
		filter   mock.PagingFilter
	}
	type wants struct {
		links influxdb.PagingLinks
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "new PagingLinks",
			args: args{
				basePath: "/api/v2/buckets",
				num:      50,
				opts: influxdb.FindOptions{
					Offset:     10,
					Limit:      10,
					Descending: true,
				},
				filter: mock.PagingFilter{
					Name: "name",
					Type: []string{"type1", "type2"},
				},
			},
			wants: wants{
				links: influxdb.PagingLinks{
					Prev: "/api/v2/buckets?descending=true&limit=10&name=name&offset=0&type=type1&type=type2",
					Self: "/api/v2/buckets?descending=true&limit=10&name=name&offset=10&type=type1&type=type2",
					Next: "/api/v2/buckets?descending=true&limit=10&name=name&offset=20&type=type1&type=type2",
				},
			},
		},
		{
			name: "new PagingLinks with empty prev link",
			args: args{
				basePath: "/api/v2/buckets",
				num:      50,
				opts: influxdb.FindOptions{
					Offset:     0,
					Limit:      10,
					Descending: true,
				},
				filter: mock.PagingFilter{
					Name: "name",
					Type: []string{"type1", "type2"},
				},
			},
			wants: wants{
				links: influxdb.PagingLinks{
					Prev: "",
					Self: "/api/v2/buckets?descending=true&limit=10&name=name&offset=0&type=type1&type=type2",
					Next: "/api/v2/buckets?descending=true&limit=10&name=name&offset=10&type=type1&type=type2",
				},
			},
		},
		{
			name: "new PagingLinks with empty next link",
			args: args{
				basePath: "/api/v2/buckets",
				num:      5,
				opts: influxdb.FindOptions{
					Offset:     10,
					Limit:      10,
					Descending: true,
				},
				filter: mock.PagingFilter{
					Name: "name",
					Type: []string{"type1", "type2"},
				},
			},
			wants: wants{
				links: influxdb.PagingLinks{
					Prev: "/api/v2/buckets?descending=true&limit=10&name=name&offset=0&type=type1&type=type2",
					Self: "/api/v2/buckets?descending=true&limit=10&name=name&offset=10&type=type1&type=type2",
					Next: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := influxdb.NewPagingLinks(tt.args.basePath, tt.args.opts, tt.args.filter, tt.args.num)

			if links.Prev != tt.wants.links.Prev {
				t.Errorf("%q. influxdb.NewPagingLinks() = %v, want %v", tt.name, links.Prev, tt.wants.links.Prev)
			}

			if links.Self != tt.wants.links.Self {
				t.Errorf("%q. influxdb.NewPagingLinks() = %v, want %v", tt.name, links.Self, tt.wants.links.Self)
			}

			if links.Next != tt.wants.links.Next {
				t.Errorf("%q. influxdb.NewPagingLinks() = %v, want %v", tt.name, links.Next, tt.wants.links.Next)
			}
		})
	}
}
