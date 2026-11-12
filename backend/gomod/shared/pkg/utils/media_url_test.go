package utils

import "testing"

func TestPublicMediaPath(t *testing.T) {
	const base = "http://localhost/goshop-media"
	cases := []struct{ name, in, want string }{
		{"relative already", "/goshop-media/products/a.png", "/goshop-media/products/a.png"},
		{"edge origin", "http://localhost/goshop-media/products/a.png", "/goshop-media/products/a.png"},
		{"stale minio origin", "http://localhost:9000/goshop-media/products/a.png", "/goshop-media/products/a.png"},
		{"stale cluster origin", "http://minio:9000/goshop-media/products/a.png", "/goshop-media/products/a.png"},
		{"external cdn untouched", "https://picsum.photos/seed/x/800/800", "https://picsum.photos/seed/x/800/800"},
		{"external unsplash untouched", "https://images.unsplash.com/photo-1?w=800", "https://images.unsplash.com/photo-1?w=800"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := PublicMediaPath(base, c.in); got != c.want {
			t.Fatalf("%s: PublicMediaPath(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}

	// A deploy behind a domain base still resolves to the same edge path.
	if got := PublicMediaPath("https://shop.example.com/goshop-media", "https://shop.example.com/goshop-media/avatars/b.png"); got != "/goshop-media/avatars/b.png" {
		t.Fatalf("domain base: got %q", got)
	}
	// No configured base: leave the value alone.
	if got := PublicMediaPath("", "http://localhost:9000/goshop-media/x.png"); got != "http://localhost:9000/goshop-media/x.png" {
		t.Fatalf("no base: got %q", got)
	}
}
