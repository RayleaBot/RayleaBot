package desktop

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type updateProgress struct {
	Stage           string `json:"stage"`
	Status          string `json:"status"`
	Version         string `json:"version"`
	PreparedID      string `json:"prepared_id"`
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
}

func (r *ReleaseFeed) downloadUpdate(parent context.Context, goos string, progress func(string)) (updateProgress, error) {
	ctx, cancel := context.WithTimeout(parent, updateDownloadTimeout)
	defer cancel()
	name := "raylea-server"
	if goos == "windows" {
		name += ".exe"
	}
	command := exec.CommandContext(ctx, filepath.Join(r.basePath, name), "-config", filepath.Join(r.basePath, "config", "user.yaml"), "update", "download", "--progress-json")
	configureChildProcess(command)
	command.WaitDelay = processKillWait
	var stderr limitedBuffer
	command.Stderr = &stderr
	pipe, err := command.StdoutPipe()
	if err != nil {
		return updateProgress{}, err
	}
	if err = command.Start(); err != nil {
		return updateProgress{}, err
	}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	var prepared updateProgress
	for scanner.Scan() {
		var event updateProgress
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		switch event.Stage {
		case "probe":
			progress("正在检测下载线路。")
		case "download":
			if event.TotalBytes > 0 {
				progress(fmt.Sprintf("正在下载更新：%d%%", min(100, event.DownloadedBytes*100/event.TotalBytes)))
			}
		case "verify":
			progress("正在解压并检查更新包，服务继续运行。")
		case "prepared":
			prepared = event
		}
	}
	if scanner.Err() != nil {
		cancel()
	}
	err = errors.Join(scanner.Err(), command.Wait())
	if ctx.Err() != nil {
		return prepared, ctx.Err()
	}
	if err != nil {
		return prepared, err
	}
	if prepared.Status == "up_to_date" {
		return prepared, nil
	}
	if prepared.Status != "update_available" || !strings.HasPrefix(prepared.PreparedID, "staging-") || strings.ContainsAny(prepared.PreparedID, "/\\") || !semverPattern.MatchString(prepared.Version) {
		return prepared, errors.New("update preparation did not return a valid result")
	}
	return prepared, nil
}
