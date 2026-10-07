package storage

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type ossTestTransport func(*http.Request) (*http.Response, error)

func (f ossTestTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestAliOSSPromotionRequestsAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name                                         string
		size                                         int64
		failPart, failComplete, conflict             bool
		completeErrorBody, unknownOutcome, publicACL bool
	}{
		{name: "single copy", size: 1 << 30},
		{name: "2GiB multipart copy", size: 2 << 30},
		{name: "source changes between parts", size: 2 << 30, failPart: true},
		{name: "complete fails", size: 2 << 30, failComplete: true},
		{name: "existing unrelated destination", size: 2 << 30, conflict: true},
		{name: "complete returns error inside HTTP 200", size: 2 << 30, completeErrorBody: true},
		{name: "complete commits before invalid response", size: 2 << 30, unknownOutcome: true},
		{name: "public destination never verified", size: 2 << 30, publicACL: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const etag = `"verified-etag"`
			var copied, parts, aborted int
			var finalExists bool
			var marker string
			respond := func(req *http.Request, status int, body string, headers http.Header) (*http.Response, error) {
				if headers == nil {
					headers = make(http.Header)
				}
				return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d", status), Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			}
			errResponse := func(req *http.Request, status int, code string) (*http.Response, error) {
				return respond(req, status, "<Error><Code>"+code+"</Code><Message>Injected test failure</Message></Error>", nil)
			}
			transport := ossTestTransport(func(req *http.Request) (*http.Response, error) {
				isSource := strings.HasSuffix(req.URL.Path, "/source")
				if req.Method == http.MethodHead {
					if !isSource && !finalExists && !tc.conflict {
						return errResponse(req, 404, "NoSuchKey")
					}
					h := make(http.Header)
					h.Set("Content-Length", strconv.FormatInt(tc.size, 10))
					h.Set("ETag", etag)
					h.Set("Content-Type", "application/pdf")
					if isSource {
						if match := req.Header.Get("If-Match"); match != "" && match != etag {
							t.Errorf("source HEAD lost If-Match: %q", match)
						}
					} else {
						h.Set("X-Oss-Meta-"+promotionMetadata, marker)
					}
					return respond(req, 200, "", h)
				}
				if isSource && req.Method == http.MethodGet {
					if req.Header.Get("Range") != "bytes=0-511" || req.Header.Get("If-Match") != etag {
						t.Errorf("inspection not bounded/conditional: %v", req.Header)
					}
					return respond(req, 206, string(bytes.Repeat([]byte{'A'}, 512)), nil)
				}
				if req.Method == http.MethodGet && req.URL.Query().Has("acl") {
					if tc.publicACL {
						return respond(req, 200, "<AccessControlPolicy><AccessControlList><Grant>public-read</Grant></AccessControlList></AccessControlPolicy>", nil)
					}
					return respond(req, 200, "<AccessControlPolicy><AccessControlList><Grant>private</Grant></AccessControlList></AccessControlPolicy>", nil)
				}
				if req.Method == http.MethodPost && req.URL.Query().Has("uploads") {
					if req.Header.Get("X-Oss-Object-Acl") != "private" || req.Header.Get("Content-Disposition") != "attachment" {
						t.Errorf("multipart metadata not private: %v", req.Header)
					}
					marker = req.Header.Get("X-Oss-Meta-" + promotionMetadata)
					if marker == "" {
						t.Error("missing verified source binding")
					}
					return respond(req, 200, "<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>final</Key><UploadId>test-upload</UploadId></InitiateMultipartUploadResult>", nil)
				}
				if req.Method == http.MethodPut {
					if req.Header.Get("X-Oss-Copy-Source-If-Match") != etag {
						t.Error("copy does not bind source ETag")
					}
					if req.URL.Query().Has("partNumber") {
						parts++
						expectRange := fmt.Sprintf("bytes=%d-%d", int64(parts-1)*(128<<20), min(int64(parts)*(128<<20), tc.size)-1)
						if req.Header.Get("X-Oss-Copy-Source-Range") != expectRange {
							t.Errorf("part range=%s expected=%s", req.Header.Get("X-Oss-Copy-Source-Range"), expectRange)
						}
						if tc.failPart && parts == 2 {
							return errResponse(req, 412, "PreconditionFailed")
						}
						return respond(req, 200, fmt.Sprintf("<CopyPartResult><ETag>part-%d</ETag></CopyPartResult>", parts), nil)
					}
					copied++
					if req.Header.Get("X-Oss-Forbid-Overwrite") != "true" || req.Header.Get("X-Oss-Object-Acl") != "private" || req.Header.Get("X-Oss-Metadata-Directive") != "REPLACE" {
						t.Errorf("unsafe copy headers: %v", req.Header)
					}
					marker = req.Header.Get("X-Oss-Meta-" + promotionMetadata)
					finalExists = true
					return respond(req, 200, "<CopyObjectResult><ETag>final-etag</ETag></CopyObjectResult>", nil)
				}
				if req.Method == http.MethodPost && req.URL.Query().Has("uploadId") {
					if req.Header.Get("X-Oss-Forbid-Overwrite") != "true" {
						t.Error("multipart complete permits overwrite")
					}
					body, err := io.ReadAll(io.LimitReader(req.Body, 8192))
					if err != nil || len(body) > 4096 {
						t.Errorf("unexpected multipart body size %d: %v", len(body), err)
					}
					if tc.failComplete {
						return errResponse(req, 400, "InvalidPart")
					}
					if tc.completeErrorBody {
						return errResponse(req, 200, "InvalidPart")
					}
					finalExists = true
					if tc.unknownOutcome {
						return respond(req, 200, "<CompleteMultipartUploadResult>", nil)
					}
					return respond(req, 200, "<CompleteMultipartUploadResult><Bucket>bucket</Bucket><Key>final</Key><ETag>complete</ETag></CompleteMultipartUploadResult>", nil)
				}
				if req.Method == http.MethodDelete && req.URL.Query().Has("uploadId") {
					aborted++
					return respond(req, 204, "", nil)
				}
				return nil, fmt.Errorf("unexpected OSS request: %s %s", req.Method, req.URL.Path)
			})
			client, err := oss.New("https://oss.invalid", "test-access", "test-secret", oss.UseCname(true), oss.HTTPClient(&http.Client{Transport: transport}))
			if err != nil {
				t.Fatal(err)
			}
			bucket, err := client.Bucket("bucket")
			if err != nil {
				t.Fatal(err)
			}
			s := &aliossServant{bucket: bucket}
			meta, err := s.InspectObject("source")
			if err != nil || meta.Size != tc.size || len(meta.Header) != 512 {
				t.Fatalf("header-only inspection: %+v,%v", meta, err)
			}
			err = s.PromoteObject("source", "final", etag)
			if tc.failPart || tc.failComplete || tc.conflict || tc.completeErrorBody || tc.publicACL {
				if err == nil || (finalExists && !tc.publicACL) {
					t.Fatalf("failure published an object: %v", err)
				}
				if !tc.conflict && aborted != 1 {
					t.Fatalf("failed multipart not aborted: %d", aborted)
				}
				if tc.conflict && (copied != 0 || parts != 0) {
					t.Fatal("existing destination was overwritten")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantParts := 0
			if tc.size > 1<<30 {
				wantParts = 16
			}
			if parts != wantParts || aborted != 0 {
				t.Fatalf("parts=%d aborted=%d", parts, aborted)
			}
			beforeCopies, beforeParts := copied, parts
			if err := s.PromoteObject("source", "final", etag); err != nil || copied != beforeCopies || parts != beforeParts {
				t.Fatalf("retry rewrote final: %v", err)
			}
		})
	}
}
