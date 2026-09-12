package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestBackendObjectCollectionCapability(t *testing.T) {
	backend, _ := newTestBackend(t, newProtocolServer())
	if _, ok := any(backend).(storage.ObjectCollectionBackend); !ok {
		t.Fatal("S3 backend cannot enumerate or conditionally delete retained catalog objects")
	}
}

const inventoryXML = `<ListBucketResult><Name>catalogs</Name><Prefix>catalog%2F</Prefix><EncodingType>url</EncodingType><KeyCount>1</KeyCount><IsTruncated>false</IsTruncated><Contents><Key>catalog%2Fa%2B%25%20%E2%98%83</Key><ETag>"v1"</ETag><Size>7</Size></Contents></ListBucketResult>`

func TestBackendObjectCollectionPages(t *testing.T) {
	var calls atomic.Int64
	backend, _ := newTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/catalogs" || query.Get("list-type") != "2" || query.Get("prefix") != "catalog/" || query.Get("max-keys") != "2" || query.Get("encoding-type") != "url" || query.Get("delimiter") != "" {
			t.Errorf("unexpected inventory request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/xml")
		if calls.Add(1) == 1 {
			if query.Get("continuation-token") != "" {
				t.Error("first request contains a continuation token")
			}
			_, _ = io.WriteString(w, strings.Replace(inventoryXML, "<IsTruncated>false</IsTruncated>", "<IsTruncated>true</IsTruncated><NextContinuationToken>opaque+/=</NextContinuationToken>", 1))
			return
		}
		if query.Get("continuation-token") != "opaque+/=" {
			t.Errorf("continuation token = %q", query.Get("continuation-token"))
		}
		_, _ = io.WriteString(w, inventoryXML)
	}))
	first, err := backend.List(t.Context(), storage.ObjectListRequest{Prefix: "catalog/", Limit: 2})
	if err != nil || len(first.Objects) != 1 || first.Next != "opaque+/=" {
		t.Fatalf("first page = %+v, %v", first, err)
	}
	want := storage.ObjectEntry{Key: "catalog/a+% ☃", Version: `"v1"`, Size: 7}
	if first.Objects[0] != want {
		t.Fatalf("entry = %+v, want %+v", first.Objects[0], want)
	}
	last, err := backend.List(t.Context(), storage.ObjectListRequest{Prefix: "catalog/", Cursor: first.Next, Limit: 2})
	if err != nil || len(last.Objects) != 1 || last.Next != "" || calls.Load() != 2 {
		t.Fatalf("last page = %+v, %v; calls = %d", last, err, calls.Load())
	}
}

func TestBackendObjectCollectionRejectsIncompletePages(t *testing.T) {
	for name, body := range map[string]string{
		"malformed XML":        "<ListBucketResult>",
		"missing completeness": strings.ReplaceAll(inventoryXML, "<IsTruncated>false</IsTruncated>", ""),
		"missing count":        strings.ReplaceAll(inventoryXML, "<KeyCount>1</KeyCount>", ""),
		"wrong count":          strings.ReplaceAll(inventoryXML, "<KeyCount>1</KeyCount>", "<KeyCount>0</KeyCount>"),
		"wrong bucket":         strings.ReplaceAll(inventoryXML, "<Name>catalogs</Name>", "<Name>other</Name>"),
		"wrong prefix":         strings.ReplaceAll(inventoryXML, "<Prefix>catalog%2F</Prefix>", "<Prefix>other%2F</Prefix>"),
		"wrong encoding":       strings.ReplaceAll(inventoryXML, "<EncodingType>url</EncodingType>", ""),
		"outside prefix":       strings.ReplaceAll(inventoryXML, "catalog%2Fa%2B%25%20%E2%98%83", "other%2Fa"),
		"malformed key":        strings.ReplaceAll(inventoryXML, "catalog%2Fa%2B%25%20%E2%98%83", "catalog%2Fa%ZZ"),
		"missing size":         strings.ReplaceAll(inventoryXML, "<Size>7</Size>", ""),
		"negative size":        strings.ReplaceAll(inventoryXML, "<Size>7</Size>", "<Size>-1</Size>"),
		"missing validator":    strings.ReplaceAll(inventoryXML, `<ETag>"v1"</ETag>`, ""),
		"wildcard validator":   strings.ReplaceAll(inventoryXML, `<ETag>"v1"</ETag>`, "<ETag>*</ETag>"),
		"missing cursor":       strings.ReplaceAll(inventoryXML, "<IsTruncated>false</IsTruncated>", "<IsTruncated>true</IsTruncated>"),
		"complete with cursor": strings.ReplaceAll(inventoryXML, "</ListBucketResult>", "<NextContinuationToken>next</NextContinuationToken></ListBucketResult>"),
		"repeated cursor":      strings.ReplaceAll(inventoryXML, "<IsTruncated>false</IsTruncated>", "<IsTruncated>true</IsTruncated><NextContinuationToken>current</NextContinuationToken>"),
		"hidden grouping":      strings.ReplaceAll(inventoryXML, "</ListBucketResult>", "<CommonPrefixes><Prefix>catalog/x/</Prefix></CommonPrefixes></ListBucketResult>"),
		"oversized XML":        strings.ReplaceAll(inventoryXML, "catalog%2Fa%2B%25%20%E2%98%83", "catalog%2F"+strings.Repeat("a", maxInventoryResponseBytes)),
	} {
		t.Run(name, func(t *testing.T) {
			backend, _ := newTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = io.WriteString(w, body)
			}))
			page, err := backend.List(t.Context(), storage.ObjectListRequest{Prefix: "catalog/", Cursor: "current", Limit: 1})
			if err == nil || len(page.Objects) != 0 || page.Next != "" {
				t.Fatalf("invalid inventory accepted: %+v, %v", page, err)
			}
		})
	}
}

func TestBackendObjectCollectionPageCardinality(t *testing.T) {
	for _, limit := range []int{1, 2} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			start := strings.Index(inventoryXML, "<Contents>")
			end := strings.Index(inventoryXML, "</Contents>") + len("</Contents>")
			body := strings.ReplaceAll(inventoryXML, "<KeyCount>1</KeyCount>", "<KeyCount>2</KeyCount>")
			body = strings.ReplaceAll(body, "</ListBucketResult>", inventoryXML[start:end]+"</ListBucketResult>")
			backend, _ := newTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) }))
			if page, err := backend.List(t.Context(), storage.ObjectListRequest{Prefix: "catalog/", Limit: limit}); err == nil || len(page.Objects) != 0 {
				t.Fatalf("excess or duplicate objects accepted: %+v, %v", page, err)
			}
		})
	}
}

func TestBackendObjectCollectionConditionalDelete(t *testing.T) {
	for _, status := range []int{204, 409, 412, 403, 404, 501} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int64
			backend, _ := newTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodDelete || r.URL.Path != "/catalogs/catalog/a" || r.Header.Get("If-Match") != `"v1"` || r.URL.Query().Has("versionId") || r.Header.Get("X-Amz-Bypass-Governance-Retention") != "" {
					t.Errorf("unexpected delete request: %s %s, condition = %q", r.Method, r.URL, r.Header.Get("If-Match"))
				}
				if status == http.StatusNoContent {
					w.WriteHeader(status)
					return
				}
				writeS3Error(w, status, map[int]string{409: "ConditionalRequestConflict", 412: "PreconditionFailed", 403: "AccessDenied", 404: "NoSuchKey", 501: "NotImplemented"}[status])
			}))
			err := backend.Delete(t.Context(), "catalog/a", `"v1"`)
			var conflict *pkgerrors.ConflictError
			switch status {
			case 204:
				if err != nil {
					t.Fatal(err)
				}
			case 409, 412:
				if !errors.As(err, &conflict) || conflict.Expected != `"v1"` {
					t.Fatalf("conditional error = %v", err)
				}
			case 404:
				if !pkgerrors.IsNotFound(err) {
					t.Fatalf("absence = %v", err)
				}
			default:
				var resource *pkgerrors.ResourceError
				if !errors.As(err, &resource) {
					t.Fatalf("resource error = %v", err)
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("request count = %d", calls.Load())
			}
		})
	}
}

func TestBackendObjectCollectionValidatesBeforeNetwork(t *testing.T) {
	var calls atomic.Int64
	backend, _ := newTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	for _, request := range []storage.ObjectListRequest{{Limit: 1}, {Prefix: "catalog/"}, {Prefix: "catalog/", Limit: -1}, {Prefix: "catalog/", Limit: storage.MaxObjectListEntries + 1}} {
		if _, err := backend.List(t.Context(), request); err == nil {
			t.Fatalf("invalid list request accepted: %+v", request)
		}
	}
	for _, version := range []string{"", " ", "*", "v1", `W/"v1"`, `"v1", "v2"`, "\"bad\r\n\""} {
		if err := backend.Delete(t.Context(), "catalog/a", version); err == nil {
			t.Fatalf("invalid validator accepted: %q", version)
		}
	}
	if err := backend.Delete(t.Context(), "", `"v1"`); err == nil {
		t.Fatal("empty deletion key accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := backend.List(ctx, storage.ObjectListRequest{Prefix: "catalog/", Limit: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled list = %v", err)
	}
	if err := backend.Delete(ctx, "catalog/a", `"v1"`); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled delete = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid operations made %d requests", calls.Load())
	}
}
