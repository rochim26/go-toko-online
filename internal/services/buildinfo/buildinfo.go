package buildinfo

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// version is computed once at startup from a hash of static assets,
// so that whenever you redeploy the binary or static files change,
// the URL query string for /static/* updates and busts CDN/browser cache.
var (
	once    sync.Once
	version string
)

func Version() string {
	once.Do(func() {
		h := sha1.New()
		// Hash the binary itself (always changes on redeploy)
		if exe, err := os.Executable(); err == nil {
			if f, err := os.Open(exe); err == nil {
				defer f.Close()
				buf := make([]byte, 64*1024)
				for {
					n, err := f.Read(buf)
					if n > 0 {
						h.Write(buf[:n])
					}
					if err != nil {
						break
					}
				}
			}
		}
		// Plus a sample of static files
		_ = filepath.Walk("static", func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			h.Write([]byte(path))
			h.Write([]byte(fmt.Sprintf("%d", info.ModTime().UnixNano())))
			h.Write([]byte(fmt.Sprintf("%d", info.Size())))
			return nil
		})
		version = hex.EncodeToString(h.Sum(nil))[:10]
		if version == "" {
			version = fmt.Sprintf("%d", time.Now().Unix())
		}
	})
	return version
}
