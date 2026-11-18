package featureapi

import "testing"

func TestUploadResponseMediaAllowlistsRasterImages(t *testing.T) {
	tests := []struct {
		give            string
		wantContentType string
		wantDisposition string
	}{
		{give: "image/png", wantContentType: "image/png", wantDisposition: "inline"},
		{give: "image/jpeg", wantContentType: "image/jpeg", wantDisposition: "inline"},
		{give: "image/gif", wantContentType: "image/gif", wantDisposition: "inline"},
		{give: "image/webp", wantContentType: "image/webp", wantDisposition: "inline"},
		{give: "image/avif", wantContentType: "image/avif", wantDisposition: "inline"},
		{give: "image/bmp", wantContentType: "image/bmp", wantDisposition: "inline"},
		{give: "image/x-icon", wantContentType: "image/x-icon", wantDisposition: "inline"},
		{give: "", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
		{give: "application/octet-stream", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
		{give: "text/html", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
		{give: "text/xml", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
		{give: "application/xhtml+xml", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
		{give: "image/svg+xml", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
		{give: "application/pdf", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
		{give: "text/plain", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
		{give: "application/javascript", wantContentType: "application/octet-stream", wantDisposition: "attachment"},
	}
	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			gotType, gotDisp := uploadResponseMedia(tt.give)
			if gotType != tt.wantContentType || gotDisp != tt.wantDisposition {
				t.Fatalf("uploadResponseMedia(%q) = (%q, %q), want (%q, %q)", tt.give, gotType, gotDisp, tt.wantContentType, tt.wantDisposition)
			}
		})
	}
}
