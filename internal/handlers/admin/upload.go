package admin

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tokoonline/app/internal/services/imageopt"
)

// GenericUpload accepts a single image file under field "file" and returns a JSON {url}
// (and renders an HTML snippet the user can copy). Used from the Settings → Store tab.
func (h *Handler) GenericUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer f.Close()
	prefix := r.FormValue("prefix")
	if prefix == "" {
		prefix = "asset"
	}
	if err := os.MkdirAll(h.UploadDir, 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	stem := fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), uuid.NewString()[:8])
	srcExt := strings.ToLower(filepath.Ext(fh.Filename))
	if srcExt == "" {
		srcExt = ".jpg"
	}
	dstPath := filepath.Join(h.UploadDir, stem+srcExt)
	finalExt, oerr := imageopt.Optimize(f, dstPath)
	if oerr != nil {
		http.Error(w, oerr.Error(), 500)
		return
	}
	url := "/uploads/" + stem + finalExt
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>Uploaded</title>
<body style="font-family:system-ui;padding:2rem;text-align:center">
<h2>Upload sukses</h2>
<p>URL:</p>
<input value="%s" style="width:100%%;padding:.5rem;font:inherit" onclick="this.select()" readonly/>
<p style="margin-top:1rem"><img src="%s" style="max-width:240px;border:1px solid #ccc;padding:8px;border-radius:8px"/></p>
<p class="muted">Copy URL di atas ke kolom Logo URL atau Favicon URL di tab Settings → Store.</p>
</body>`, url, url)
}
