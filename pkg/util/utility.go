package util

import (
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/valyala/fasthttp"
)

func SaveFileToDisk(fh *multipart.FileHeader, saveTo string) error {
	saveDir := filepath.Dir(saveTo)
	if _, err := os.Stat(saveDir); err != nil {
		os.MkdirAll(saveDir, 0755)
	}

	return fasthttp.SaveMultipartFile(fh, saveTo)
}
