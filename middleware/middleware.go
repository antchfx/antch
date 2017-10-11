package middleware

import (
	"github.com/antchfx/antch"
)

var DefaultDownloaderMiddlewares = []antch.DownloaderMiddleware{
	downloadDelayMiddleware(),
	httpCompressionMiddleware(),
	redirectMiddleware(),
	cookieMiddleware(),
}

func redirectMiddleware() antch.DownloaderMiddleware {
	return func(next antch.Downloader) antch.Downloader {
		return &Redirect{Next: next}
	}
}

func cookieMiddleware() antch.DownloaderMiddleware {
	return func(next antch.Downloader) antch.Downloader {
		return &Cookie{Next: next}
	}
}

func httpCompressionMiddleware() antch.DownloaderMiddleware {
	return func(next antch.Downloader) antch.Downloader {
		return &HttpCompression{Next: next}
	}
}

func downloadDelayMiddleware() antch.DownloaderMiddleware {
	return func(next antch.Downloader) antch.Downloader {
		return &DownloadDelay{Next: next}
	}
}
