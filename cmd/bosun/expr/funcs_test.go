package expr

import (
	"fmt"
	"testing"
	"time"

	"bosun.org/opentsdb"

	"github.com/influxdata/influxdb/client"
)

type exprInOut struct {
	expr           string
	out            Results
	shouldParseErr bool
}

func testExpression(eio exprInOut) error {
	e, err := New(eio.expr, builtins)
	if eio.shouldParseErr {
		if err == nil {
			return fmt.Errorf("no error when expected error on %v", eio.expr)
		}
		return nil
	}
	if err != nil {
		return err
	}
	backends := &Backends{
		InfluxConfig: client.Config{},
	}
	providers := &BosunProviders{}
	r, _, err := e.Execute(backends, providers, nil, queryTime, 0, false)
	if err != nil {
		return err
	}
	if _, err := eio.out.Equal(r); err != nil {
		return err
	}
	return nil
}

func TestDuration(t *testing.T) {
	d := exprInOut{
		`d("1h")`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Scalar(3600),
				},
			},
		},
		false,
	}
	err := testExpression(d)
	if err != nil {
		t.Error(err)
	}
}

func TestToDuration(t *testing.T) {
	inputs := []int{
		0,
		1,
		60,
		3600,
		86400,
		604800,
		31536000,
	}
	outputs := []string{
		"0ms",
		"1s",
		"1m",
		"1h",
		"1d",
		"1w",
		"1y",
	}

	for i := range inputs {
		d := exprInOut{
			fmt.Sprintf(`tod(%d)`, inputs[i]),
			Results{
				Results: ResultSlice{
					&Result{
						Value: String(outputs[i]),
					},
				},
			},
			false,
		}
		err := testExpression(d)
		if err != nil {
			t.Error(err)
		}
	}
}

func TestUngroup(t *testing.T) {
	dictum := `series("foo=bar", 0, ungroup(last(series("foo=baz", 0, 1))))`
	err := testExpression(exprInOut{
		dictum,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "bar"},
				},
			},
		},
		false,
	})

	if err != nil {
		t.Error(err)
	}
}

func TestMap(t *testing.T) {
	err := testExpression(exprInOut{
		`map(series("test=test", 0, 1, 1, 3), expr(v()+1))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 2,
						time.Unix(1, 0): 4,
					},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	})
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`avg(map(series("test=test", 0, 1, 1, 3), expr(v()+1)))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Number(3),
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	})
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`1 + avg(map(series("test=test", 0, 1, 1, 3), expr(v()+1))) + 1`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Number(5),
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	})
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`max(map(series("test=test", 0, 1, 1, 3), expr(v()+v())))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Number(6),
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	})
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`map(series("test=test", 0, -2, 1, 3), expr(1+1))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 2,
						time.Unix(1, 0): 2,
					},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	})
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`map(series("test=test", 0, -2, 1, 3), expr(abs(v())))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 2,
						time.Unix(1, 0): 3,
					},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	})
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`map(series("test=test", 0, -2, 1, 3), expr(series("test=test", 0, v())))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 2,
						time.Unix(1, 0): 3,
					},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		true, // expect parse error here, series result not valid as TypeNumberExpr
	})
	if err != nil {
		t.Error(err)
	}
}

func TestMerge(t *testing.T) {
	seriesA := `series("foo=bar", 0, 1)`
	seriesB := `series("foo=baz", 0, 1)`
	err := testExpression(exprInOut{
		fmt.Sprintf("merge(%v, %v)", seriesA, seriesB),
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "bar"},
				},
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "baz"},
				},
			},
		},
		false,
	})
	if err != nil {
		t.Error(err)
	}

	//Should Error due to identical groups in merge
	err = testExpression(exprInOut{
		fmt.Sprintf("merge(%v, %v)", seriesA, seriesA),
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "bar"},
				},
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "bar"},
				},
			},
		},
		false,
	})
	if err == nil {
		t.Errorf("error expected due to identical groups in merge but did not get one")
	}
}

func TestTimedelta(t *testing.T) {
	for _, i := range []struct {
		input    string
		expected Series
	}{
		{
			`timedelta(series("foo=bar", 1466133600, 1, 1466133610, 1, 1466133710, 1))`,
			Series{
				time.Unix(1466133610, 0): 10,
				time.Unix(1466133710, 0): 100,
			},
		},
		{
			`timedelta(series("foo=bar", 1466133600, 1))`,
			Series{
				time.Unix(1466133600, 0): 0,
			},
		},
	} {

		err := testExpression(exprInOut{
			i.input,
			Results{
				Results: ResultSlice{
					&Result{
						Value: i.expected,
						Group: opentsdb.TagSet{"foo": "bar"},
					},
				},
			},
			false,
		})

		if err != nil {
			t.Error(err)
		}
	}
}
