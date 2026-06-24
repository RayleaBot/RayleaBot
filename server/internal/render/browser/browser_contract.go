package browser

import "context"

type Document struct {
	Template          string
	Theme             string
	Output            string
	BaseURL           string
	Width             int
	Height            int
	AutoHeight        bool
	DeviceScaleFactor float64
	HTML              string
}

type Runner interface {
	Render(ctx context.Context, doc Document) ([]byte, error)
}

func IsChromiumRunner(runner Runner) bool {
	_, ok := runner.(*chromiumRunner)
	return ok
}
