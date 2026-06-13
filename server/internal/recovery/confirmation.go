package recovery

import (
	"strings"
	"time"
)

func ConfirmSkippedPlugins(summary CompatibilitySummary, reviewIDs []string, operatorID, note, taskID string) (CompatibilitySummary, []string, error) {
	reviewIDs = dedupeStrings(reviewIDs)
	if len(reviewIDs) == 0 {
		return summary, nil, &UnknownReviewIDsError{}
	}

	indexByReviewID := map[string]int{}
	for index, skipped := range summary.SkippedPlugins {
		if strings.TrimSpace(skipped.ReviewID) != "" {
			indexByReviewID[skipped.ReviewID] = index
		}
	}

	unknown := make([]string, 0, len(reviewIDs))
	for _, reviewID := range reviewIDs {
		if _, ok := indexByReviewID[reviewID]; !ok {
			unknown = append(unknown, reviewID)
		}
	}
	if len(unknown) > 0 {
		return summary, nil, &UnknownReviewIDsError{ReviewIDs: unknown}
	}

	operatorID = strings.TrimSpace(operatorID)
	note = strings.TrimSpace(note)
	confirmedAt := time.Now().UTC().Format(time.RFC3339)
	newlyConfirmed := make([]string, 0, len(reviewIDs))
	auditItems := make([]AuditItem, 0, len(reviewIDs))

	for _, reviewID := range reviewIDs {
		skipped := &summary.SkippedPlugins[indexByReviewID[reviewID]]
		if skipped.ReviewStatus == reviewStatusConfirmed {
			continue
		}
		skipped.ReviewStatus = reviewStatusConfirmed
		skipped.ReviewedAt = confirmedAt
		skipped.ReviewedBy = operatorID
		newlyConfirmed = append(newlyConfirmed, reviewID)
		auditItems = append(auditItems, AuditItem{
			ReviewID:   skipped.ReviewID,
			PluginID:   skipped.PluginID,
			ReasonCode: skipped.ReasonCode,
			Summary:    skipped.Summary,
			Version:    skipped.Version,
		})
	}

	machineIssues := filterMachineIssues(summary.Issues)
	pendingSkipped := pendingSkippedPlugins(summary.SkippedPlugins)
	summary.Issues = append(machineIssues, issuesForSkippedPlugins(pendingSkipped)...)
	if len(summary.Issues) == 0 {
		summary.Issues = nil
	}
	summary.Status = recoveryStatus(machineIssues, pendingSkipped)
	if summary.Status == "compatible" {
		summary.ManualActions = nil
		summary.NextSteps = nil
	} else {
		summary.ManualActions = buildManualActions(machineIssues, pendingSkipped)
		summary.NextSteps = buildNextSteps(machineIssues, pendingSkipped)
	}
	if len(newlyConfirmed) > 0 {
		summary.UpdatedAt = confirmedAt
		summary.Audit = trimAuditEntries(append([]AuditEntry{{
			TaskID:     strings.TrimSpace(taskID),
			CreatedAt:  confirmedAt,
			OperatorID: operatorID,
			Note:       note,
			Items:      auditItems,
		}}, summary.Audit...))
	}
	return summary, newlyConfirmed, nil
}

type reviewConfirmation struct {
	ReviewedAt string
	ReviewedBy string
}

func confirmedReviewLookup(summary CompatibilitySummary) map[string]reviewConfirmation {
	lookup := map[string]reviewConfirmation{}
	for _, skipped := range summary.SkippedPlugins {
		if skipped.ReviewStatus != reviewStatusConfirmed || strings.TrimSpace(skipped.ReviewID) == "" {
			continue
		}
		lookup[skipped.ReviewID] = reviewConfirmation{
			ReviewedAt: skipped.ReviewedAt,
			ReviewedBy: skipped.ReviewedBy,
		}
	}
	for _, entry := range summary.Audit {
		for _, item := range entry.Items {
			if strings.TrimSpace(item.ReviewID) == "" {
				continue
			}
			if _, exists := lookup[item.ReviewID]; exists {
				continue
			}
			lookup[item.ReviewID] = reviewConfirmation{
				ReviewedAt: entry.CreatedAt,
				ReviewedBy: entry.OperatorID,
			}
		}
	}
	return lookup
}
