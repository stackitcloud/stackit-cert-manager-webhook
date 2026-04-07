package repository_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	stackitdnsclient "github.com/stackitcloud/stackit-sdk-go/services/dns/v1api"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func getTestServer(t *testing.T) *httptest.Server { //nolint:funlen // This is a test helper
	t.Helper()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	// Case FetchZone success
	mux.HandleFunc("/v1/projects/1234/zones", func(w http.ResponseWriter, r *http.Request) {
		getZonesResponseSuccess(t, w)
	})
	// Case FetchZone no zones
	mux.HandleFunc("/v1/projects/0000/zones", func(w http.ResponseWriter, r *http.Request) {
		getZonesResponseNoZones(t, w)
	})
	// Case FetchZone failure
	mux.HandleFunc("/v1/projects/5678/zones", func(w http.ResponseWriter, r *http.Request) {
		failureResponse(t, w)
	})
	// Case FetchRRSetForZone success
	mux.HandleFunc(
		"/v1/projects/1234/zones/1234/rrsets",
		func(w http.ResponseWriter, r *http.Request) {
			getRRSetResponseSuccess(t, w)
		},
	)
	// Case FetchRRSetForZone failure
	mux.HandleFunc(
		"/v1/projects/1234/zones/5678/rrsets",
		func(w http.ResponseWriter, r *http.Request) {
			failureResponse(t, w)
		},
	)
	// Case FetchRRSetForZone error not found
	mux.HandleFunc(
		"/v1/projects/1234/zones/9999/rrsets",
		func(w http.ResponseWriter, r *http.Request) {
			getRRSetResponseNoRRSets(t, w)
		},
	)
	// Case CreateRRSet success
	mux.HandleFunc(
		"/v1/projects/1234/zones/0000/rrsets",
		func(w http.ResponseWriter, r *http.Request) {
			postRRSetResponseSuccess(t, w)
		},
	)
	// Case CreateRRSet failure
	mux.HandleFunc(
		"/v1/projects/1234/zones/1111/rrsets",
		func(w http.ResponseWriter, r *http.Request) {
			failureResponse(t, w)
		},
	)
	// Case UpdateRRSet success
	mux.HandleFunc(
		"/v1/projects/1234/zones/2222/rrsets/0000",
		func(w http.ResponseWriter, r *http.Request) {
			patchRRSetResponseSuccess(t, w)
		},
	)
	// Case UpdateRRSet failure
	mux.HandleFunc(
		"/v1/projects/1234/zones/3333/rrsets/1111",
		func(w http.ResponseWriter, r *http.Request) {
			failureResponse(t, w)
		},
	)
	// Case DeleteRRSet success
	mux.HandleFunc(
		"/v1/projects/1234/zones/1234/rrsets/2222",
		func(w http.ResponseWriter, r *http.Request) {
			patchRRSetResponseSuccess(t, w)
		},
	)
	// Case DeleteRRSet failure
	mux.HandleFunc(
		"/v1/projects/1234/zones/1234/rrsets/3333",
		func(w http.ResponseWriter, r *http.Request) {
			failureResponse(t, w)
		},
	)
	// Case DeleteRRSet 400 return
	mux.HandleFunc(
		"/v1/projects/1234/zones/1234/rrsets/4444",
		func(w http.ResponseWriter, r *http.Request) {
			deleteRRSetResponse400(t, w)
		},
	)
	// Case DeleteRRSet 404 return
	mux.HandleFunc(
		"/v1/projects/1234/zones/1234/rrsets/5555",
		func(w http.ResponseWriter, r *http.Request) {
			deleteRRSetResponse404(t, w)
		},
	)

	return server
}

func getZonesResponseSuccess(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	zones := stackitdnsclient.ListZonesResponse{
		ItemsPerPage: int32(10),
		Message:      ptr.To("success"),
		TotalItems:   int32(1),
		TotalPages:   int32(1),
		Zones: []stackitdnsclient.Zone{
			{Id: "1234", DnsName: "test.com"},
		},
	}

	successResponseBytes, err := json.Marshal(zones)
	assert.NoError(t, err)

	w.WriteHeader(http.StatusOK)
	w.Write(successResponseBytes)
}

func getZonesResponseNoZones(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	zones := stackitdnsclient.ListZonesResponse{
		ItemsPerPage: int32(10),
		Message:      ptr.To("success"),
		TotalItems:   int32(1),
		TotalPages:   int32(1),
		Zones:        []stackitdnsclient.Zone{},
	}

	successResponseBytes, err := json.Marshal(zones)
	assert.NoError(t, err)

	w.WriteHeader(http.StatusOK)
	w.Write(successResponseBytes)
}

func failureResponse(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
}

func getRRSetResponseSuccess(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	rrSets := stackitdnsclient.ListRecordSetsResponse{
		ItemsPerPage: int32(20),
		Message:      ptr.To("success"),
		RrSets: []stackitdnsclient.RecordSet{
			{
				Name: "test.com.",
				Type: "TXT",
				Ttl:  int32(300),
				Records: []stackitdnsclient.Record{
					{Content: "_acme-challenge.test.com"},
				},
				Id: "1234",
			},
		},
		TotalItems: int32(2),
		TotalPages: int32(1),
	}

	successResponseBytes, err := json.Marshal(rrSets)
	assert.NoError(t, err)

	w.WriteHeader(http.StatusOK)
	w.Write(successResponseBytes)
}

func getRRSetResponseNoRRSets(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	rrSets := stackitdnsclient.ListRecordSetsResponse{
		ItemsPerPage: int32(20),
		Message:      ptr.To("success"),
		RrSets:       []stackitdnsclient.RecordSet{},
		TotalItems:   int32(2),
		TotalPages:   int32(1),
	}

	successResponseBytes, err := json.Marshal(rrSets)
	assert.NoError(t, err)

	w.WriteHeader(http.StatusOK)
	w.Write(successResponseBytes)
}

func postRRSetResponseSuccess(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	rrSets := stackitdnsclient.RecordSetResponse{
		Message: ptr.To("success"),
		Rrset: stackitdnsclient.RecordSet{
			Active:  ptr.To(true),
			Comment: ptr.To("created by webhook"),
			Id:      "1234",
			Name:    "test.com.",
		},
	}

	successResponseBytes, err := json.Marshal(rrSets)
	assert.NoError(t, err)

	w.WriteHeader(http.StatusAccepted)
	w.Write(successResponseBytes)
}

func patchRRSetResponseSuccess(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	writeResponseMessageSuccess(t, w, http.StatusAccepted)
}

func deleteRRSetResponse400(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	writeResponseMessageSuccess(t, w, http.StatusBadRequest)
}

func deleteRRSetResponse404(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	writeResponseMessageSuccess(t, w, http.StatusNotFound)
}

func writeResponseMessageSuccess(t *testing.T, w http.ResponseWriter, statusCode int) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	rrSets := stackitdnsclient.Message{Message: ptr.To("success")}

	successResponseBytes, err := json.Marshal(rrSets)
	assert.NoError(t, err)

	w.WriteHeader(statusCode)
	w.Write(successResponseBytes)
}
