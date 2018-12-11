package main

import (
    "fmt"
	"log"
	"github.com/PuerkitoBio/goquery"
)

func main() {
	doc, err := goquery.NewDocument("http://www.apsim.info/BugTracker/bugs.aspx")
	if err != nil {
	log.Fatal(err)
	}
	
	doc.Find("td.bugd>a").Each(func(index int, item *goquery.Selection) {
		title := item.Text()
		fmt.Println(title)
	})
}