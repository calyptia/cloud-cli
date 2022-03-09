package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/calyptia/api/types"
	cloud "github.com/calyptia/api/types"
)

func Test_newCmdGetAggregators(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := &bytes.Buffer{}
		cmd := newCmdGetAggregators(configWithMock(nil))
		cmd.SetOutput(got)

		err := cmd.Execute()
		wantEq(t, nil, err)
		wantEq(t, "NAME AGE\n", got.String())

		t.Run("with_ids", func(t *testing.T) {
			got.Reset()
			cmd.SetArgs([]string{"--show-ids"})

			err := cmd.Execute()
			wantEq(t, nil, err)
			wantEq(t, "ID NAME AGE\n", got.String())
		})
	})

	t.Run("error", func(t *testing.T) {
		want := errors.New("internal error")
		cmd := newCmdGetAggregators(configWithMock(&ClientMock{
			AggregatorsFunc: func(ctx context.Context, projectID string, params types.AggregatorsParams) (cloud.Aggregators, error) {
				return cloud.Aggregators{}, want
			},
		}))
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true

		got := cmd.Execute()
		wantEq(t, want, got)
	})

	t.Run("ok", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		want := cloud.Aggregators{
			Items: []cloud.Aggregator{{
				ID:        "id_1",
				Name:      "name_1",
				CreatedAt: now.Add(-time.Hour),
			}, {
				ID:        "id_2",
				Name:      "name_2",
				CreatedAt: now.Add(time.Minute * -10),
			}},
		}
		got := &bytes.Buffer{}
		cmd := newCmdGetAggregators(configWithMock(&ClientMock{
			AggregatorsFunc: func(ctx context.Context, projectID string, params types.AggregatorsParams) (cloud.Aggregators, error) {
				wantNoEq(t, nil, params.Last)
				wantEq(t, uint64(2), *params.Last)
				return want, nil
			},
		}))
		cmd.SetOutput(got)
		cmd.SetArgs([]string{"--last=2"})

		err := cmd.Execute()
		wantEq(t, nil, err)
		wantEq(t, ""+
			"NAME   AGE\n"+
			"name_1 1 hour\n"+
			"name_2 10 minutes\n", got.String())

		t.Run("with_ids", func(t *testing.T) {
			got.Reset()
			cmd.SetArgs([]string{"--show-ids"})

			err := cmd.Execute()
			wantEq(t, nil, err)
			wantEq(t, ""+
				"ID   NAME   AGE\n"+
				"id_1 name_1 1 hour\n"+
				"id_2 name_2 10 minutes\n", got.String())
		})

		t.Run("json", func(t *testing.T) {
			want, err := json.Marshal(want.Items)
			wantEq(t, nil, err)

			got.Reset()
			cmd.SetArgs([]string{"--output-format=json"})

			err = cmd.Execute()
			wantEq(t, nil, err)
			wantEq(t, string(want)+"\n", got.String())
		})
	})
}

func Test_aggregatorsKeys(t *testing.T) {
	tt := []struct {
		name  string
		given []cloud.Aggregator
		want  []string
	}{
		{
			given: []cloud.Aggregator{{ID: "id-1", Name: "name-1"}, {ID: "id-2", Name: "name-2"}},
			want:  []string{"name-1", "name-2"},
		},
		{
			given: []cloud.Aggregator{{ID: "id-1", Name: "name"}, {ID: "id-2", Name: "name"}},
			want:  []string{"id-1", "id-2"},
		},
		{
			given: []cloud.Aggregator{{ID: "id-1", Name: "name"}, {ID: "id-2", Name: "name"}, {ID: "id-3", Name: "other-name"}},
			want:  []string{"id-1", "id-2", "other-name"},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if got := aggregatorsKeys(tc.given); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Aggregators.Keys(%+v) = %v, want %v", tc.given, got, tc.want)
			}
		})
	}
}
