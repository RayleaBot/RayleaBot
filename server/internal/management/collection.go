package management

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
)

func readCollectionQuery(w http.ResponseWriter, r *http.Request) (pagination.Query, bool) {
	query, err := pagination.Parse(r.URL.Query())
	if err != nil {
		httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
		return pagination.Query{}, false
	}
	return query, true
}

func readCollectionChoice(w http.ResponseWriter, r *http.Request, name string, allowed ...string) (string, bool) {
	values, present := r.URL.Query()[name]
	if !present {
		return "", true
	}
	if len(values) == 1 {
		for _, value := range allowed {
			if values[0] == value {
				return value, true
			}
		}
	}
	httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
	return "", false
}
