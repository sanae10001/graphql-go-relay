package graphqlrelay

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/graph-gophers/graphql-go"
)

const (
	ContentTypeJSON    = "application/json"
	ContentTypeGraphQL = "application/graphql"
)

// Used to customize response
// eg: custom error struct or fill Response.Extensions
type OnResponse func(r *graphql.Response) interface{}

type Handler struct {
	Schema     *graphql.Schema
	pretty     bool
	onResponse OnResponse
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "GraphQL only supports POST requests.", http.StatusMethodNotAllowed)
		return
	}

	if r.Body == nil {
		http.Error(w, "No body provided.", http.StatusBadRequest)
		return
	}

	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}

	// Check Content-Type
	contentTypeStr := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentTypeStr, ContentTypeJSON):
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			http.Error(w, "POST body is invalid JSON.", http.StatusBadRequest)
			return
		}
	case strings.HasPrefix(contentTypeStr, ContentTypeGraphQL):
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			if err == bytes.ErrTooLarge {
				http.Error(w, "POST body is too large.", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "POST body is invalid.", http.StatusInternalServerError)
			}
			return
		}
		params.Query = string(data)
	default:
		http.Error(w, "Not supported content type.", http.StatusBadRequest)
		return
	}

	if params.Query == "" {
		http.Error(w, "Must provide query string.", http.StatusBadRequest)
		return
	}

	var resp interface{}
	response := h.Schema.Exec(r.Context(), params.Query, params.OperationName, params.Variables)

	if h.onResponse != nil {
		resp = h.onResponse(response)
	} else {
		resp = response
	}

	// Process response
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if h.pretty {
		encoder.SetIndent("", "\t")
	}
	if err := encoder.Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func NewHandler(schema *graphql.Schema, pretty bool, onResponse OnResponse) *Handler {
	if schema == nil {
		panic("nil GraphQL schema")
	}

	return &Handler{
		Schema:     schema,
		pretty:     pretty,
		onResponse: onResponse,
	}
}
