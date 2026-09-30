package catalog

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/sgash708/skillctl/internal/manifest"
	"github.com/sgash708/skillctl/internal/pluginclient"
)

type fakeRunner struct {
	res pluginclient.Result
	err error
}

func (f fakeRunner) Run(ctx context.Context, name string, args ...string) (pluginclient.Result, error) {
	return f.res, f.err
}

func TestFetch(t *testing.T) {
	mp := manifest.Marketplace{
		Name:  "example-skills",
		Owner: manifest.Owner{Name: "example-org"},
		Plugins: []manifest.Plugin{
			{Name: "example-skill", Source: "./example-skill", Description: "スリープ抑止", Version: "0.0.0"},
		},
	}
	mpJSON := `{"name":"example-skills","owner":{"name":"example-org"},"plugins":[{"name":"example-skill","source":"./example-skill","description":"スリープ抑止","version":"0.0.0"}]}`
	encoded := base64.StdEncoding.EncodeToString([]byte(mpJSON))

	tests := []struct {
		name    string
		runner  fakeRunner
		want    []manifest.Plugin
		wantErr bool
	}{
		{
			name:   "正常系",
			runner: fakeRunner{res: pluginclient.Result{ExitCode: 0, Stdout: `{"content":"` + encoded + `"}`}},
			want:   mp.Plugins,
		},
		{
			name:    "gh api失敗",
			runner:  fakeRunner{err: errors.New("gh not found")},
			wantErr: true,
		},
		{
			name:    "非ゼロ終了",
			runner:  fakeRunner{res: pluginclient.Result{ExitCode: 1, Stderr: "404"}},
			wantErr: true,
		},
		{
			name:    "contentが不正なbase64",
			runner:  fakeRunner{res: pluginclient.Result{ExitCode: 0, Stdout: `{"content":"***not-base64***"}`}},
			wantErr: true,
		},
		{
			name:    "contentが不正なJSON",
			runner:  fakeRunner{res: pluginclient.Result{ExitCode: 0, Stdout: `{"content":"` + base64.StdEncoding.EncodeToString([]byte("not json")) + `"}`}},
			wantErr: true,
		},
		{
			name:    "payloadのJSONが不正",
			runner:  fakeRunner{res: pluginclient.Result{ExitCode: 0, Stdout: `{invalid json}`}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Fetch(context.Background(), tt.runner, "example-org", "skills")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len(got) = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
