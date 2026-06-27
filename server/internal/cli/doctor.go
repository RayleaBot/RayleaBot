package cli

func runDoctor(cmd Command) int {
	report := BuildDoctorReport(cmd)

	hasProblems := false
	for _, issue := range report.Issues {
		if issue.Severity != "ok" {
			cmd.Logger.Warn(issue.Summary, "code", issue.Code)
			hasProblems = true
		} else {
			cmd.Logger.Info(issue.Summary, "code", issue.Code)
		}
	}

	if hasProblems {
		cmd.Logger.Warn("服务自检完成，发现需要处理的问题")
		return 1
	}
	cmd.Logger.Info("服务自检完成，所有检查通过")
	return 0
}
