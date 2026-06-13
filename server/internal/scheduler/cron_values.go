package scheduler

func toSet(vals []int) map[int]bool {
	m := make(map[int]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

func dedupSort(vals []int, min, max int) []int {
	seen := make(map[int]bool, len(vals))
	result := make([]int, 0, len(vals))
	for _, v := range vals {
		if v >= min && v <= max && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	// Simple insertion sort — field values are small.
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j] < result[j-1]; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}
