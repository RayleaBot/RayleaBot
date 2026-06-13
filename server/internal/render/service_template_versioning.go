package render

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

func newRevisionID(templateID, digest string) string {
	templateID = strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(strings.TrimSpace(templateID))
	if len(digest) > 8 {
		digest = digest[:8]
	}
	sequence := atomic.AddUint64(&revisionCounter, 1)
	return fmt.Sprintf("rev_%s_%s_%s_%06d", templateID, time.Now().UTC().Format("20060102T150405000000000"), digest, sequence)
}
