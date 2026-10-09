package main

import (
	"net"
	"reflect"
	"testing"
)

func TestParseS3Path(t *testing.T) {
	tests := []struct {
		path, bucket, key string
	}{
		{"s3://satellite-certs/ssl-watch", "satellite-certs", "ssl-watch"},
		{"s3://le.afdevops.com/ssl-watch", "le.afdevops.com", "ssl-watch"},
		{"s3://bucket/some/dir", "bucket", "some/dir"},
		{"s3://bucket/", "bucket", ""},
		{"s3://bucket", "bucket", ""},
		// A local directory is not an S3 path, so NewApp reads config files from disk.
		{"/etc/ssl-watch", "", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		bucket, key := ParseS3Path(tt.path)
		if bucket != tt.bucket || key != tt.key {
			t.Errorf("ParseS3Path(%q) = (%q, %q), want (%q, %q)", tt.path, bucket, key, tt.bucket, tt.key)
		}
	}
}

func TestIsIPv4(t *testing.T) {
	tests := map[string]bool{
		"127.0.0.1":      true,
		"192.168.0.7":    true,
		"::1":            false,
		"2001:db8::1":    false,
		"::ffff:1.2.3.4": false,
	}
	for address, want := range tests {
		if got := IsIPv4(address); got != want {
			t.Errorf("IsIPv4(%q) = %v, want %v", address, got, want)
		}
	}
}

func TestStrToIp(t *testing.T) {
	got := StrToIp([]string{"127.0.0.1", "not-an-ip", "", "::1"})
	want := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StrToIp() = %v, want %v", got, want)
	}
}
