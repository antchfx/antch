package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antchfx/antch"
	"github.com/antchfx/antch/middleware"
	"github.com/antchfx/xquery/html"
)

func amazonSpider(q antch.Queue) antch.Spider {
	return antch.SpiderFunc(func(ctx context.Context, resp *http.Response) error {
		fmt.Println(resp.Request.URL)
		// Parse HTTP response as HTML document.
		doc, err := antch.ParseHTML(resp)
		if err != nil {
			return err
		}
		// extract all links from HTML document using XPath.
		for _, n := range htmlquery.Find(doc, "//a[@href]") {
			urlstr := htmlquery.SelectAttr(n, "href")
			if u, err := resp.Request.URL.Parse(urlstr); err == nil {
				q.Enqueue(u.String())
			}
		}
		return nil
	})
}

func main() {
	exitCh := make(chan int)
	var startURL = "https://www.amazon.com/"

	// Declare a Queue is used by Crawler.
	queue := &antch.SimpleHeapQueue{}
	queue.Enqueue(startURL)

	// Declare a new instance of Downloader.
	downloader := &antch.DownloaderStack{}
	// Registers a new middleware for Downloader
	for _, mid := range middleware.DefaultDownloaderMiddlewares {
		downloader.UseMiddleware(mid)
	}

	// Declare a spider to handle all receives HTTP response.
	spider := amazonSpider(queue)
	var crawler = &antch.Crawler{
		MaxWorkers:      1,
		DownloadHandler: downloader,
		MessageHandler:  spider,
	}
	crawler.Run(queue)

	<-exitCh
	crawler.Stop()
}
