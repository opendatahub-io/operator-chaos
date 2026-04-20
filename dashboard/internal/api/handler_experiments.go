package api

import (
	"net/http"
	"strconv"

	"github.com/opendatahub-io/operator-chaos/dashboard/internal/store"
)

func (s *Server) handleListExperiments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := store.ListFilter{
		Namespace: q.Get("namespace"),
		Operator:  q.Get("operator"),
		Component: q.Get("component"),
		Type:      q.Get("type"),
		Verdict:   q.Get("verdict"),
		Phase:     q.Get("phase"),
		Search:    q.Get("search"),
		Sort:      q.Get("sort"),
		Order:     q.Get("order"),
		Page:      1,
		PageSize:  10,
	}

	if v := q.Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			filter.Page = p
		}
	}
	if v := q.Get("pageSize"); v != "" {
		if ps, err := strconv.Atoi(v); err == nil {
			filter.PageSize = ps
		}
	}
	if filter.PageSize > 500 {
		filter.PageSize = 500
	}
	if since, err := parseSince(r); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	} else if since != nil {
		filter.Since = since
	}

	result, err := s.store.List(filter)
	if err != nil {
		internalError(w, "list experiments", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":      result.Items,
		"totalCount": result.TotalCount,
	})
}

func (s *Server) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	namespace := pathSegment(r, "namespace")
	name := pathSegment(r, "name")

	exp, err := s.store.Get(namespace, name)
	if err != nil {
		internalError(w, "get experiment", err)
		return
	}
	if exp == nil {
		writeError(w, http.StatusNotFound, "experiment not found")
		return
	}

	writeJSON(w, http.StatusOK, exp)
}
