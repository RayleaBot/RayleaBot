//go:build !windows

package cli

func platformDoctorIssues() []DoctorIssue {
	return nil
}
