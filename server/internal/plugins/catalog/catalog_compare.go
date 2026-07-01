package catalog

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func pluginStateChanged(current plugins.Snapshot, next plugins.Snapshot) bool {
	return current.RegistrationState != next.RegistrationState ||
		current.DesiredState != next.DesiredState ||
		current.RuntimeState != next.RuntimeState ||
		current.DisplayState != next.DisplayState ||
		!deadLetterEqual(current.DeadLetter, next.DeadLetter) ||
		!commandsEqual(current.Commands, next.Commands)
}

func deadLetterEqual(left *plugins.DeadLetterSnapshot, right *plugins.DeadLetterSnapshot) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return left.EnteredAt.Equal(right.EnteredAt) &&
		left.CrashCount == right.CrashCount &&
		left.LastErrorCode == right.LastErrorCode &&
		left.LastErrorMessage == right.LastErrorMessage
}

func commandsEqual(left []plugins.Command, right []plugins.Command) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name ||
			left[index].Description != right[index].Description ||
			left[index].Usage != right[index].Usage ||
			left[index].Permission != right[index].Permission ||
			left[index].CommandSource != right[index].CommandSource ||
			left[index].MatchPattern != right[index].MatchPattern ||
			left[index].DeclarationID != right[index].DeclarationID ||
			!stringSlicesEqual(left[index].Aliases, right[index].Aliases) {
			return false
		}
	}
	return true
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
