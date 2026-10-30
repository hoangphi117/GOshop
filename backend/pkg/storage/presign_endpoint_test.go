package storage

import "testing"

func TestPresignEndpointFor(t *testing.T) {
	cases := []struct {
		name string
		cfg  S3Config
		want string
	}{
		{
			name: "edge base with bucket path collapses to the public host",
			cfg:  S3Config{Endpoint: "http://minio:9000", Bucket: "goshop-media", PublicBaseURL: "http://localhost/goshop-media"},
			want: "http://localhost",
		},
		{
			name: "legacy /minio prefix base also collapses to host",
			cfg:  S3Config{Endpoint: "http://minio:9000", Bucket: "goshop-media", PublicBaseURL: "http://localhost/minio/goshop-media"},
			want: "http://localhost",
		},
		{
			name: "trailing slash tolerated",
			cfg:  S3Config{Endpoint: "http://minio:9000", Bucket: "goshop-media", PublicBaseURL: "https://shop.example.com/goshop-media/"},
			want: "https://shop.example.com",
		},
		{
			name: "host-only base kept as-is",
			cfg:  S3Config{Endpoint: "http://minio:9000", Bucket: "goshop-media", PublicBaseURL: "https://shop.example.com"},
			want: "https://shop.example.com",
		},
		{
			name: "no public base falls back to internal endpoint",
			cfg:  S3Config{Endpoint: "http://minio:9000", Bucket: "goshop-media"},
			want: "http://minio:9000",
		},
		{
			name: "r2 keeps the S3 API endpoint (public domain cannot sign)",
			cfg:  S3Config{Endpoint: "https://acc.r2.cloudflarestorage.com", Bucket: "goshop-media", PublicBaseURL: "https://pub-xxx.r2.dev"},
			want: "https://acc.r2.cloudflarestorage.com",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := presignEndpointFor(tc.cfg); got != tc.want {
				t.Fatalf("presignEndpointFor() = %q, want %q", got, tc.want)
			}
		})
	}
}
