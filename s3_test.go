package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

// fakeS3 serves ListObjects and GetObject for one bucket, with path-style URLs.
// Each object's ETag is its key, so tests can tell objects apart.
func fakeS3(t *testing.T, bucket string, objects map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+bucket)
		switch {
		case r.Method != http.MethodGet:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		case path == "" || path == "/":
			prefix := r.URL.Query().Get("prefix")
			var contents strings.Builder
			for key, body := range objects {
				if strings.HasPrefix(key, prefix) {
					fmt.Fprintf(&contents, `<Contents><Key>%s</Key><ETag>"%s"</ETag><Size>%d</Size></Contents>`, key, key, len(body))
				}
			}
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+
				`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`+
				`<Name>%s</Name><Prefix>%s</Prefix><IsTruncated>false</IsTruncated>%s</ListBucketResult>`,
				bucket, prefix, contents.String())
		default:
			body, ok := objects[strings.TrimPrefix(path, "/")]
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`)
				return
			}
			fmt.Fprint(w, body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newS3TestApp returns an App that reads its config from s3://bucket/ssl-watch on a fake S3 server.
func newS3TestApp(t *testing.T, objects map[string]string) *App {
	t.Helper()
	srv := fakeS3(t, "bucket", objects)
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String(srv.URL),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials("id", "secret", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	app := newTestApp()
	app.config.S3Bucket, app.config.S3Key = ParseS3Path("s3://bucket/ssl-watch")
	app.S3Session = sess
	return app
}

var s3Objects = map[string]string{
	"ssl-watch/a.conf": `{"a": {"domains": {"a.com": []}}}`,
	"ssl-watch/b.conf": `{"b": {"domains": {"b.com": []}}}`,
	// Ignored: wrong suffix.
	"ssl-watch/c.json": `{"c": {"domains": {"c.com": []}}}`,
	// Ignored: outside the prefix.
	"other/d.conf": `{"d": {"domains": {"d.com": []}}}`,
}

func TestGetS3ConfigHashes(t *testing.T) {
	app := newS3TestApp(t, s3Objects)

	hashes, err := app.GetS3ConfigHashes()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"ssl-watch/a.conf": `"ssl-watch/a.conf"`, "ssl-watch/b.conf": `"ssl-watch/b.conf"`}
	if len(hashes) != len(want) {
		t.Fatalf("GetS3ConfigHashes() = %v, want %v", hashes, want)
	}
	for key, etag := range want {
		if hashes[key] != etag {
			t.Errorf("GetS3ConfigHashes()[%q] = %q, want %q", key, hashes[key], etag)
		}
	}
}

func TestReadS3File(t *testing.T) {
	app := newS3TestApp(t, s3Objects)

	raw, err := app.ReadS3File("ssl-watch/a.conf")
	if err != nil || string(raw) != s3Objects["ssl-watch/a.conf"] {
		t.Errorf("ReadS3File(ssl-watch/a.conf) = (%q, %v)", raw, err)
	}
	if _, err := app.ReadS3File("ssl-watch/missing.conf"); err == nil {
		t.Error("ReadS3File(ssl-watch/missing.conf) returned no error")
	}
}

func TestReloadConfigFromS3(t *testing.T) {
	app := newS3TestApp(t, s3Objects)
	app.ReloadConfig()

	domains := app.services.ListDomains()
	if len(domains) != 2 {
		t.Fatalf("ReloadConfig read domains %v, want a.com and b.com", domains)
	}
	for _, domain := range []string{"a.com", "b.com"} {
		if _, ok := app.services.GetServiceName(domain); !ok {
			t.Errorf("ReloadConfig did not read %s", domain)
		}
	}

	// ReloadConfig records the ETags it read, so the auto-reload loop sees no change.
	current, err := app.GetS3ConfigHashes()
	if err != nil {
		t.Fatal(err)
	}
	if app.S3ConfigsChanged(current) {
		t.Errorf("right after ReloadConfig, S3ConfigsChanged() = true; recorded %v, current %v", app.S3Configs, current)
	}
}
