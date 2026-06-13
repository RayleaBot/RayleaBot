package tasks

func isTaskError(err error, target **TaskError) bool {
	te, ok := err.(*TaskError)
	if ok {
		*target = te
	}
	return ok
}

func statusPtr(s Status) *Status { return &s }
func strPtr(s string) *string    { return &s }
func intP(i int) *int            { return &i }
