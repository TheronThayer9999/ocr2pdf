package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner chạy một chương trình ngoài và trả về stdout.
// Tách riêng để engine có thể test bằng runner giả.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// ExecRunner chạy lệnh thật bằng os/exec.
type ExecRunner struct{}

// Run thực thi lệnh; lỗi trả về kèm stderr để dễ debug.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("%s: quá thời gian chờ", name)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return stdout.String(), nil
}
