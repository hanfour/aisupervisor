package installer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

// downloadFile downloads a URL to a local file path, reporting progress via onProgress.
// The progress callback receives a value between 0.0 and 1.0.
func downloadFile(ctx context.Context, url, dest string, onProgress func(float64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	totalSize, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer out.Close()

	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("writing file: %w", writeErr)
			}
			downloaded += int64(n)
			if totalSize > 0 && onProgress != nil {
				onProgress(float64(downloaded) / float64(totalSize))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("reading response: %w", readErr)
		}
	}

	if onProgress != nil {
		onProgress(1.0)
	}
	return nil
}
