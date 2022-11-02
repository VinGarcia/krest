package krest

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	tt "github.com/vingarcia/krest/internal/testtools"
)

func TestMultipartRequests(t *testing.T) {
	ctx := context.Background()

	var handler http.HandlerFunc
	mockServer := httptest.NewServer(&handler)
	defer mockServer.Close()

	type testCase struct {
		desc  string
		parts map[string]io.Reader

		expectedFilenames   map[string]string
		expectedPartHeaders map[string]textproto.MIMEHeader
		expectedPartBodies  map[string]string
	}

	for _, test := range []testCase{
		{
			desc: "should read parsed items correctly",
			parts: map[string]io.Reader{
				"fakeItemName1": MultipartItem(strings.NewReader("fakeBlob1"), "text/plain"),
				"fakeItemName2": MultipartItem(strings.NewReader("fakeBlob2"), "text/html"),
			},
			expectedFilenames: map[string]string{},
			expectedPartHeaders: map[string]textproto.MIMEHeader{
				"fakeItemName1": textproto.MIMEHeader{
					"Content-Type": []string{
						"text/plain",
					},
				},
				"fakeItemName2": textproto.MIMEHeader{
					"Content-Type": []string{
						"text/html",
					},
				},
			},
			expectedPartBodies: map[string]string{
				"fakeItemName1": "fakeBlob1",
				"fakeItemName2": "fakeBlob2",
			},
		},
		{
			desc: "should read parsed files correctly",
			parts: map[string]io.Reader{
				"fakeItemName1": MultipartFile(strings.NewReader("fakeBlob1"), "fakeFilename1"),
				"fakeItemName2": MultipartFile(strings.NewReader("fakeBlob2"), "fakeFilename2"),
			},
			expectedFilenames: map[string]string{
				"fakeItemName1": "fakeFilename1",
				"fakeItemName2": "fakeFilename2",
			},
			expectedPartHeaders: map[string]textproto.MIMEHeader{
				"fakeItemName1": textproto.MIMEHeader{
					"Content-Type": []string{
						"application/octet-stream",
					},
				},
				"fakeItemName2": textproto.MIMEHeader{
					"Content-Type": []string{
						"application/octet-stream",
					},
				},
			},
			expectedPartBodies: map[string]string{
				"fakeItemName1": "fakeBlob1",
				"fakeItemName2": "fakeBlob2",
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			partFilenames := map[string]string{}
			partHeaders := map[string]textproto.MIMEHeader{}
			partBodies := map[string]string{}
			handler = func(w http.ResponseWriter, r *http.Request) {
				_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
				tt.AssertNoErr(t, err)

				mr := multipart.NewReader(r.Body, params["boundary"])
				for {
					p, err := mr.NextPart()
					if err == io.EOF {
						return
					}
					tt.AssertNoErr(t, err)

					partBody, err := io.ReadAll(p)
					tt.AssertNoErr(t, err)

					if p.FileName() != "" {
						partFilenames[p.FormName()] = p.FileName()
					}
					partHeaders[p.FormName()] = p.Header
					partBodies[p.FormName()] = string(partBody)
				}
			}

			client := New(30 * time.Second)
			resp, err := client.Post(ctx, mockServer.URL, RequestData{
				Body: test.parts,
			})
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, resp.StatusCode, 200)

			tt.AssertEqual(t, partFilenames, test.expectedFilenames)
			tt.AssertEqual(t, partBodies, test.expectedPartBodies)
			for partName, header := range test.expectedPartHeaders {
				for key := range header {
					tt.AssertNotEqual(t, partHeaders[partName], nil)
					tt.AssertEqual(t, partHeaders[partName][key], header[key])
				}
			}
		})
	}
}

func TestMultipartStream(t *testing.T) {
	t.Run("should stream normal readers correctly", func(t *testing.T) {
		stream, contentType, err := newMultipartStream(map[string]io.Reader{
			"item1": strings.NewReader(`{"fake":"json"}`),
			"item2": strings.NewReader(`================ other payload ==================`),
		})
		tt.AssertEqual(t, nil, err)

		boundary := stream.multipartWriter.Boundary()
		tt.AssertEqual(t, true, strings.Contains(contentType, `multipart/form-data;`))
		tt.AssertEqual(t, true, strings.Contains(contentType, `boundary=`+boundary))

		// Reading he payload little by little
		// to make sure we are processing
		// the pauses correctly:
		var payload string
		buf := make([]byte, 10)
		var n int
		for err == nil {
			n, err = stream.Read(buf)
			payload += string(buf[:n])
		}
		tt.AssertEqual(t, io.EOF, err)
		tt.AssertEqual(t, 1, strings.Count(payload, `{"fake":"json"}`))
		tt.AssertEqual(t, 1, strings.Count(payload, `================ other payload ==================`))
		tt.AssertEqual(t, 3, strings.Count(payload, boundary))
		tt.AssertEqual(t, 1, strings.Count(payload, `name="item1"`))
		tt.AssertEqual(t, 1, strings.Count(payload, `name="item2"`))
		tt.AssertEqual(t, 0, strings.Count(payload, `Content-Type:`))
	})

	t.Run("should stream items with Content-Type correctly", func(t *testing.T) {
		stream, contentType, err := newMultipartStream(map[string]io.Reader{
			"item1": MultipartItem(strings.NewReader(`{"fake":"json"}`), "application/json"),
			"item2": strings.NewReader(`================ other payload ==================`),
		})
		tt.AssertEqual(t, nil, err)

		boundary := stream.multipartWriter.Boundary()
		tt.AssertEqual(t, true, strings.Contains(contentType, `multipart/form-data;`))
		tt.AssertEqual(t, true, strings.Contains(contentType, `boundary=`+boundary))

		// Reading he payload little by little
		// to make sure we are processing
		// the pauses correctly:
		var payload string
		buf := make([]byte, 10)
		var n int
		for err == nil {
			n, err = stream.Read(buf)
			payload += string(buf[:n])
		}
		tt.AssertEqual(t, io.EOF, err)
		tt.AssertEqual(t, 1, strings.Count(payload, `{"fake":"json"}`))
		tt.AssertEqual(t, 1, strings.Count(payload, `================ other payload ==================`))
		tt.AssertEqual(t, 3, strings.Count(payload, boundary))
		tt.AssertEqual(t, 1, strings.Count(payload, `name="item1"`))
		tt.AssertEqual(t, 1, strings.Count(payload, `name="item2"`))
		tt.AssertEqual(t, 1, strings.Count(payload, `Content-Type:`))
		tt.AssertEqual(t, 1, strings.Count(payload, `Content-Type: application/json`))
	})

	t.Run("should stream files correctly", func(t *testing.T) {
		stream, contentType, err := newMultipartStream(map[string]io.Reader{
			"item1": strings.NewReader(`{"fake":"json"}`),
			"item2": MultipartFile(strings.NewReader(`================ other payload ==================`), "fake-filename"),
		})
		tt.AssertEqual(t, nil, err)

		boundary := stream.multipartWriter.Boundary()
		tt.AssertEqual(t, true, strings.Contains(contentType, `multipart/form-data;`))
		tt.AssertEqual(t, true, strings.Contains(contentType, `boundary=`+boundary))

		// Reading he payload little by little
		// to make sure we are processing
		// the pauses correctly:
		var payload string
		buf := make([]byte, 10)
		var n int
		for err == nil {
			n, err = stream.Read(buf)
			payload += string(buf[:n])
		}
		tt.AssertEqual(t, io.EOF, err)
		tt.AssertEqual(t, 1, strings.Count(payload, `{"fake":"json"}`))
		tt.AssertEqual(t, 1, strings.Count(payload, `================ other payload ==================`))
		tt.AssertEqual(t, 3, strings.Count(payload, boundary))
		tt.AssertEqual(t, 1, strings.Count(payload, `name="item1"`))
		tt.AssertEqual(t, 1, strings.Count(payload, `name="item2"`))
		tt.AssertEqual(t, 1, strings.Count(payload, `filename="fake-filename"`))
		tt.AssertEqual(t, 1, strings.Count(payload, `Content-Type:`))
		tt.AssertEqual(t, 1, strings.Count(payload, `Content-Type: application/octet-stream`))
	})
}
