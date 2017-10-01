package main

import "github.com/antchfx/antch"

func main() {
	var startURLs = []string{
		"https://www.amazon.com/",
		"https://www.reddit.com/",
		"https://news.ycombinator.com/news",
	}
	// Declare a Queue is used by Crawler.
	queue := &antch.SimpleHeapQueue{}
	for _, URL := range startURLs {
		queue.Enqueue(URL)
	}

	// Start to crawling website.
	antch.DefaultCrawler.Run(queue)
}
