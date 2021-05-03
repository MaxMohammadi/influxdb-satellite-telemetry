package spectests

import (
	"fmt"
	"path/filepath"
	"runtime"
)

var aggregateFuncNames = []string{
	"count",
	"mean",
	"sum",
}

func AggregateTest(fn func(name string) (stmt, want string)) Fixture {
	_, file, line, _ := runtime.Caller(1)
	fixture := &collection{
		file: filepath.Base(file),
		line: line,
	}

	for _, name := range aggregateFuncNames {
		stmt, want := fn(name)
		fixture.Add(stmt, want)
	}
	return fixture
}

func init() {
	RegisterFixture(
		AggregateTest(func(name string) (stmt, want string) {
			return fmt.Sprintf(`SELECT %s(value) FROM db0..cpu`, name),
				`package main

` + fmt.Sprintf(`from(bucketID: "%s")`, bucketID.String()) + `
	|> range(start: 1677-09-21T00:12:43.145224194Z, stop: 2262-04-11T23:47:16.854775806Z)
	|> filter(fn: (r) => r._measurement == "cpu" and r._field == "value")
	|> group(columns: ["_measurement", "_start", "_stop", "_field"], mode: "by")
	|> keep(columns: ["_measurement", "_start", "_stop", "_field", "_time", "_value"])
	|> ` + name + `()
	|> map(fn: (r) => ({r with _time: 1970-01-01T00:00:00Z}))
	|> rename(columns: {_value: "` + name + `"})
	|> yield(name: "0")
`
		}),
	)
}
