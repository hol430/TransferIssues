package main

import (
    "fmt"
	"log"
	"github.com/PuerkitoBio/goquery"
	"strings"
	"strconv"
	"unicode"
	"time"
)

const rootUrl = "https://www.apsim.info/BugTracker/"
const dateFormat = "2006-01-02 3:4 PM"

func parseInt(str string) int64 {
	result, err := strconv.ParseInt(str, 0, 64)
	if err != nil {
		log.Fatal(err)
	}
	return result
}

func getComments(url string) (comments []Comment) {
	threadDoc, err := goquery.NewDocument(url)
	if err != nil {
		log.Fatal(err)
	}
	threadDoc.Find(".cmt").Each(func(i int, commentData *goquery.Selection) {
		commentText := strings.TrimSpace(commentData.Find("table:nth-child(2)").Text())
		commentMetadata := strings.TrimSpace(commentData.Find("table:nth-child(1)").Text())
		commentMetadata = strings.Map(func(r rune) rune {
			if r == '\u00a0' {
				return '\u0020'
			}
			return r
		}, commentMetadata)
		splitMetadata := strings.Split(commentMetadata, " ")
		var attachments []Attachment
		if splitMetadata[0] == "file" {
			attachmentInfo := commentData.Find(".pst")
			attachmentNameNode := commentData.Find("img").Parent().Next()
			// For now, ignore whether the href exists or not.
			attachmentUrl, _ := attachmentNameNode.Next().Attr("href")
			attachments = append(attachments, Attachment {
				name : attachmentNameNode.Text(),
				size : parseInt(strings.Split(strings.Map(func(r rune) rune {
					if unicode.IsSpace(r) {
						return '\u0020'
					}
					return r
				}, attachmentInfo.Last().Text()), " ")[1]),
				url: rootUrl + attachmentUrl,
			})
		}
		commentDate, err := time.Parse(dateFormat , splitMetadata[6] + " " + splitMetadata[7] + " " + strings.Trim(splitMetadata[8], ","))
		if err != nil {
			log.Fatal(err)
		}
		comment := Comment {
			id: parseInt(splitMetadata[1]),
			author: splitMetadata[4],
			date: commentDate,
			text: commentText,
			attachments: attachments,
		}
		comments = append(comments, comment)
	})
	return
}

func main() {
	doc, err := goquery.NewDocument(rootUrl + "bugs.aspx")
	if err != nil {
		log.Fatal(err)
	}
	var bugs []Bug
	doc.Find("table.bugt tr").Each(func(index int, row *goquery.Selection) {
		// Skip the first two rows of the table, as these don't contain bugs.
		if index > 1 {
			// The fourth cell in the table contains an anchor element
			// which links to the bug's thread.
			threadUrl, exists := row.Find("td:nth-child(4)>a").Attr("href")
			var comments []Comment
			if exists {
				comments = getComments(rootUrl + threadUrl)
			}
			bugDate, err := time.Parse(dateFormat , row.Find("td:nth-child(8)").Text())
			if err != nil {
				log.Fatal(err)
			}
			
			bug := Bug {
				id: parseInt(row.Find("td:nth-child(1)").Text()),
				description: row.Find("td:nth-child(4)").Text(),
				priority: row.Find("td:nth-child(2)").Text(),
				status: row.Find("td:nth-child(3)").Text(),
				project: row.Find("td:nth-child(5)").Text(),
				category: row.Find("td:nth-child(6)").Text(),
				reporter: row.Find("td:nth-child(7)").Text(),
				date: bugDate,
				assignee: row.Find("td:nth-child(9)").Text(),
				comments : comments,
			}
			bugs = append(bugs, bug)
			fmt.Println(bug.ToString())
			fmt.Println("--------------------------------------------")
			
		}
	})
	fmt.Println(len(bugs))
}