package pluginclient

import (
	"context"
	"testing"
)

func TestExecRunner_Run(t *testing.T) {
	tests := []struct {
		name       string
		cmdName    string
		args       []string
		wantStdout string
		wantExit   int
		wantErr    bool
	}{
		{
			name:       "正常終了・標準出力を取得",
			cmdName:    "sh",
			args:       []string{"-c", "echo hello"},
			wantStdout: "hello\n",
			wantExit:   0,
		},
		{
			name:     "非ゼロ終了コード",
			cmdName:  "sh",
			args:     []string{"-c", "exit 7"},
			wantExit: 7,
		},
		{
			name:    "存在しないコマンド",
			cmdName: "definitely-not-a-real-binary-xyz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ExecRunner{}
			got, err := r.Run(context.Background(), tt.cmdName, tt.args...)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if got.ExitCode != -1 {
					t.Errorf("ExitCode = %d, want -1", got.ExitCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", got.Stdout, tt.wantStdout)
			}
			if got.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", got.ExitCode, tt.wantExit)
			}
		})
	}
}
