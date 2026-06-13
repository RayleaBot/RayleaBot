package logging

import (
	"context"
	"errors"
)

func (q *SpoolQueue) Flush(ctx context.Context, repository Repository) (SpoolFlushResult, error) {
	if q == nil || q.path == "" {
		return SpoolFlushResult{}, nil
	}
	if repository == nil {
		return SpoolFlushResult{}, errors.New("management log repository is required")
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	lines, err := q.readLines()
	if err != nil {
		return SpoolFlushResult{}, err
	}
	if len(lines) == 0 {
		return SpoolFlushResult{}, nil
	}

	result := SpoolFlushResult{}
	remaining := make([][]byte, 0, len(lines))
	for index, line := range lines {
		select {
		case <-ctx.Done():
			remaining = append(remaining, line)
			remaining = append(remaining, lines[index+1:]...)
			result.Pending = len(remaining)
			if rewriteErr := q.rewrite(remaining); rewriteErr != nil {
				return result, errors.Join(ctx.Err(), rewriteErr)
			}
			return result, ctx.Err()
		default:
		}

		summary, decodeErr := decodeSpoolRecord(line)
		if decodeErr != nil {
			if quarantineErr := q.appendQuarantine(line); quarantineErr != nil {
				remaining = append(remaining, line)
				remaining = append(remaining, lines[index+1:]...)
				result.Pending = len(remaining)
				if rewriteErr := q.rewrite(remaining); rewriteErr != nil {
					return result, errors.Join(decodeErr, quarantineErr, rewriteErr)
				}
				return result, errors.Join(decodeErr, quarantineErr)
			}
			result.Quarantined++
			continue
		}

		if err := repository.SaveSummary(ctx, summary); err != nil {
			remaining = append(remaining, line)
			remaining = append(remaining, lines[index+1:]...)
			result.Pending = len(remaining)
			if rewriteErr := q.rewrite(remaining); rewriteErr != nil {
				return result, errors.Join(err, rewriteErr)
			}
			return result, err
		}

		result.Flushed++
	}

	if err := q.rewrite(remaining); err != nil {
		return result, err
	}
	return result, nil
}
