package recovery

func trimAuditEntries(entries []AuditEntry) []AuditEntry {
	if len(entries) == 0 {
		return nil
	}
	if len(entries) > maxAuditEntries {
		entries = entries[:maxAuditEntries]
	}
	cloned := make([]AuditEntry, 0, len(entries))
	for _, entry := range entries {
		items := make([]AuditItem, 0, len(entry.Items))
		for _, item := range entry.Items {
			items = append(items, item)
		}
		cloned = append(cloned, AuditEntry{
			TaskID:     entry.TaskID,
			CreatedAt:  entry.CreatedAt,
			OperatorID: entry.OperatorID,
			Note:       entry.Note,
			Items:      items,
		})
	}
	return cloned
}

func cloneIssues(issues []CompatibilityIssue) []CompatibilityIssue {
	if len(issues) == 0 {
		return nil
	}
	cloned := make([]CompatibilityIssue, 0, len(issues))
	for _, issue := range issues {
		cloned = append(cloned, CompatibilityIssue{
			Code:        issue.Code,
			Severity:    issue.Severity,
			Summary:     issue.Summary,
			Remediation: issue.Remediation,
		})
	}
	return cloned
}
