package main

import (
	"reflect"
	"testing"
)

func TestBuildArgv(t *testing.T) {
	tests := []struct {
		name     string
		template string
		args     []string
		want     []string
		wantErr  bool
	}{
		{"no placeholder", "run_df -h", nil, []string{"run_df", "-h"}, false},
		{"unquoted whole word splits", "sudo journalctl $ARG$ --no-pager", []string{"-u foo -n 100"},
			[]string{"sudo", "journalctl", "-u", "foo", "-n", "100", "--no-pager"}, false},
		{"unquoted honours quotes in arg", "journalctl $ARG$", []string{`--since "30 days ago" -g 'a|b'`},
			[]string{"journalctl", "--since", "30 days ago", "-g", "a|b"}, false},
		{"single-quoted stays one word", "check_procs -a '$ARG$'", []string{"x' -u root '"},
			[]string{"check_procs", "-a", "x' -u root '"}, false},
		{"double-quoted stays one word", `check_procs -a "$ARG$"`, []string{`a b" -c "1`},
			[]string{"check_procs", "-a", `a b" -c "1`}, false},
		{"embedded in word", "check_disk -w '$ARG$'% -c '$ARG$'%", []string{"10", "5"},
			[]string{"check_disk", "-w", "10%", "-c", "5%"}, false},
		{"embedded unquoted stays one word", "echo pre$ARG$post", []string{"a b"},
			[]string{"echo", "prea bpost"}, false},
		{"order", "echo $ARG$-$ARG$-$ARG$", []string{"a", "b", "c"}, []string{"echo", "a-b-c"}, false},
		{"arg10 vs arg1 markers", "echo $ARG$ $ARG$ $ARG$ $ARG$ $ARG$ $ARG$ $ARG$ $ARG$ $ARG$ $ARG$ $ARG$ '$ARG$'",
			[]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "x y"},
			[]string{"echo", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "x y"}, false},
		{"not enough args", "echo $ARG$ $ARG$", []string{"a"}, nil, true},
		{"unbalanced quote in arg", "journalctl $ARG$", []string{`--since "x`}, nil, true},
		{"unbalanced quote in quoted arg is literal", "echo '$ARG$'", []string{`it's`}, []string{"echo", "it's"}, false},
		{"escaped quote in template", `echo \'$ARG$`, []string{"a b"}, []string{"echo", "'a b"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildArgv(tt.template, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
