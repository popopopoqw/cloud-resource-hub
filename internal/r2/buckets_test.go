package r2

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestListBucketsFollowsContinuationToken(t *testing.T) {
	client := New("account", "access", "secret", "")
	requests := 0
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if !strings.HasPrefix(req.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Fatal("missing signed authorization")
		}
		body := `<ListAllMyBucketsResult><Buckets><Bucket><Name>zeta</Name></Bucket></Buckets><IsTruncated>true</IsTruncated><NextContinuationToken>next-page</NextContinuationToken></ListAllMyBucketsResult>`
		if requests == 2 {
			if req.URL.Query().Get("continuation-token") != "next-page" {
				t.Fatalf("continuation token = %q", req.URL.Query().Get("continuation-token"))
			}
			body = `<ListAllMyBucketsResult><Buckets><Bucket><Name>alpha</Name></Bucket></Buckets><IsTruncated>false</IsTruncated></ListAllMyBucketsResult>`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	buckets, err := client.ListBuckets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(buckets) != 2 || buckets[0] != "alpha" || buckets[1] != "zeta" {
		t.Fatalf("requests=%d buckets=%v", requests, buckets)
	}
}

func TestListBucketsViaAPIFiltersUnsupportedJurisdictions(t *testing.T) {
	client := New("account", "", "", "")
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("missing API token")
		}
		body := `{"success":true,"result":{"buckets":[{"name":"default-bucket","jurisdiction":"default"},{"name":"eu-bucket","jurisdiction":"eu"}]},"result_info":{"cursor":""}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	buckets, err := client.ListBucketsViaAPI(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0] != "default-bucket" {
		t.Fatalf("buckets=%v", buckets)
	}
}
