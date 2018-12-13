package main

import (
    "fmt"
	"log"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"os"
	"strings"
	"strconv"
	"unicode"
	"time"
)

const rootUrl = "https://www.apsim.info/BugTracker/"
const dateFormat = "1/2/2006 3:4:5 PM"
const commentDateFormat =  "2006-1-2 3:4 PM"
const shortCommentDateFormat = "2006-1-2"
// 1/2/2006 3:4:5 PM

// Parses an int64 from a string.
func parseInt(str string) int64 {
	result, err := strconv.ParseInt(str, 0, 64)
	if err != nil {
		log.Fatal(err)
	}
	return result
}

// Returns true if the slice contains any of the values
func contains(arr []string, values ...string) bool {
	for _, arrValue := range arr {
		for _, match := range values {
			if arrValue == match {
				return true
			}
		}
	}
	return false
}

// Gets the comments for a particular bug ID.
// Returns a slice of comments.
func getComments(bugId int64) (comments []Comment) {
	threadDoc, err := goquery.NewDocument(rootUrl + "edit_bug.aspx?id=" + strconv.Itoa(int(bugId)))
	if err != nil {
		log.Fatal(err)
	}
	threadDoc.Find(".cmt").Each(func(i int, commentData *goquery.Selection) {
		commentText := strings.TrimSpace(commentData.Find("table:nth-child(2)").Text())
		
		// Comment metadata is the sentence at the top of the comment which gives the
		// Comment ID, author, and date.
		commentMetadata := strings.TrimSpace(commentData.Find("span.pst").First().Text())
		
		// Replace any pesky non-breaking spaces with normal spaces, so we can
		// split the string on the space character.
		commentMetadata = strings.Map(func(r rune) rune {
			if r == '\u00a0' {
				return '\u0020'
			}
			return r
		}, commentMetadata)
		splitMetadata := strings.Split(commentMetadata, " ")
		
		// Check if the post contains any attachments.
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
		
		// There is one comment(!) on one bug which is different to all other
		// comments on all other bugs. The date reported for this comment
		// is just yyyy-m-d (e.g. no time component). To work around this,
		// check if the word which would normally hold the date actually
		// contains a colon ":". If it doesn't contain a colon, this must be
		// the special comment.
		n := len(splitMetadata) - 1
		var commentDate time.Time
		if strings.Contains(splitMetadata[n - 4], ":") {
			commentDate, err = time.Parse(commentDateFormat , splitMetadata[n - 5] + " " + splitMetadata[n - 4] + " " + strings.Trim(splitMetadata[n - 3], ","))
		} else {
			commentDate, err = time.Parse(shortCommentDateFormat, strings.Trim(splitMetadata[n - 3], ","))
		}
		if err != nil {
			fmt.Printf("Error parsing date for Bug #%d\n", bugId)
			fmt.Printf("commentMetadata: %v\n", commentMetadata)
			fmt.Printf("n: %d; date: %v\n", n, splitMetadata[n - 5])
			log.Fatal(err)
		}
		
		comment := Comment {
			id: parseInt(splitMetadata[1]),
			author: splitMetadata[4],
			date: commentDate,
			text: commentText,
			attachments: attachments,
		}
		
		// Skip this particular comment...
		if comment.id != 33552 {
			// Prepend the comment to the list of comments.
			comments = append([]Comment { comment }, comments...)
		}
	})
	return
}

// Loads the list of bugs.
func loadBugList() *goquery.Document {
	// We want to load print_bugs.aspx, however this page relies on cookies
	// which are set in bugs.aspx. Therefore, we load bugs.aspx and reuse the
	// cookies in the response for the request to print_bugs.aspx.
	
	// Load bugs.aspx
	response, err := http.Get(rootUrl + "bugs.aspx?qu_id=1")
	if err != nil {
		log.Fatal(err)
	}
	doc, err := goquery.NewDocumentFromResponse(response)
	if err != nil {
		log.Fatal(err)
	}
	
	// Create a new request to print_bugs.aspx (but don't invoke the request
	// just yet).
	request, err := http.NewRequest("GET", rootUrl + "print_bugs.aspx", nil)
	if err != nil {
		log.Fatal(err)
	}
	
	// Copy all cookies from the response to the original request over to the
	// request to print_bugs.aspx
	for _, cookie := range response.Cookies() {
		request.AddCookie(cookie)
	}
	
	// Invoke the request to print_bugs.aspx
	client := &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	
	// Load a goquery document from the response.
	doc, err = goquery.NewDocumentFromResponse(response)
	if err != nil {
		log.Fatal(err)
	}
	return doc
}

func main() {
	var verbose bool = !contains(os.Args, "-q", "--quiet")
	
	doc := loadBugList()
	var bugs []Bug
	bugRows := doc.Find("table.bugt tr")
	numBugs := bugRows.IndexOfSelection(bugRows.Last())
	bugRows.Each(func(index int, row *goquery.Selection) {
		if verbose {
			fmt.Printf("Processing bugs...%.2f%%\r", float64(index) / float64(numBugs) * 100.0)
		}
		// Skip the first row of the table, as it doesn't contain bugs.
		if index > 0 {
			bugId := parseInt(row.Find("td:nth-child(1)").Text())
			bugDate, err := time.Parse(dateFormat , row.Find("td:nth-child(8)").Text())
			if err != nil {
				fmt.Printf("Error parsing date in bug #%d", bugId)
				// Bail immediately if we fail to parse a date.
				log.Fatal(err)
			}
			
			bug := Bug {
				id: bugId,
				description: row.Find("td:nth-child(4)").Text(),
				priority: row.Find("td:nth-child(2)").Text(),
				status: row.Find("td:nth-child(3)").Text(),
				project: row.Find("td:nth-child(5)").Text(),
				category: row.Find("td:nth-child(6)").Text(),
				author: strings.Replace(row.Find("td:nth-child(7)").Text(), ":", "", -1),
				date: bugDate,
				assignee: row.Find("td:nth-child(9)").Text(),
				comments : getComments(bugId),
			}
			bugs = append([]Bug { bug }, bugs...)
		}
	})
	if verbose {
		fmt.Printf("Processing bugs...Finished!\n")
	}
	for _, bug := range bugs {
		fmt.Println(bug.ToString())
		fmt.Println("--------------------------------------------")
	}
}