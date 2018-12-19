package main

import (
	"bufio"
    "fmt"
	"github.com/octokit/go-octokit/octokit"
	"github.com/jlaffaye/ftp"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"math/rand"
	"strings"
	"strconv"
	"time"
)

// 1/2/2006 3:4:5 PM
const dateFormat = "1/2/2006 3:4:5 PM"
// Dates on comments are printed differently to dates on bugs.
const commentDateFormat =  "2006-1-2 3:4 PM"
// There is one comment which is a special snowflake.
const shortCommentDateFormat = "2006-1-2"

var blacklistedComments = []int64{ 686, 688, 32121, 32124, 32125, 32284, 32287, 32295, 32311, 32331, 32355, 32380, 32394, 32396, 32397, 32420, 32479, 32544, 32605, 32683, 32717, 32774, 32775, 32848, 32767, 32938, 32939, 32984, 33012, 33438, 33552, 33888, 33926, 33950, 33951, 34103, 34108, 34109, 34113, 34116, 34128, 34131, 34132, 33525, 33542, 33666, 33945, 33955, 34122, 34134 }

// Returns the smaller of two integers.
// x: The first integer.
// y: The second integer.
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// Creates a directory if it doesn't exist.
func CreateDirIfNotExist(dir string) {
      if _, err := os.Stat(dir); os.IsNotExist(err) {
              err = os.MkdirAll(dir, 0755)
              if err != nil {
                      panic(err)
              }
      }
}

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

// Replaces all non-breaking spaces with regular spaces.
// str: input string.
func stripNonBreakingSpaces(str string) string {
	return strings.Map(func(r rune) rune {
		if r == '\u00a0' {
			return '\u0020'
		}
		return r
	}, str)
}

// Loads the page which contains the list of bugs.
// rootUrl: Root URL of the bug tracker website.
// Must contain trailing forward slash.
// e.g. https://www.apsim.info/BugTracker/
func loadBugList(rootUrl string) *goquery.Document {
	// We want to load print_bugs.aspx, however this page relies on cookies
	// which are set in bugs.aspx. Therefore, we load bugs.aspx and reuse the
	// cookies in the response for the request to print_bugs.aspx.
	
	// Load bugs.aspx
	response, err := http.Get(rootUrl + "bugs.aspx?qu_id=1")
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
	doc, err := goquery.NewDocumentFromResponse(response)
	if err != nil {
		log.Fatal(err)
	}
	return doc
}

// Checks if a comment is blacklisted.
// id: ID of the comment.
func isBlackListed(id int64) bool {
	for _, b := range blacklistedComments {
		if id == b {
			return true
		}
	}
	return false
}

// Reads a secret from a file on disk.
// file: Path to the file on disk.
func getSecret(file string) string {
	secret, err := ioutil.ReadFile(file)
	
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(string(secret))
}

// Gets the comments for a particular bug ID.cls
// Returns a slice of comments.
// rootUrl: Root URL of the bug tracker website.
// Must contain trailing forward slash.
// e.g. https://www.apsim.info/BugTracker/
func getComments(rootUrl string, bugId int) (comments []Comment) {
	threadDoc, err := goquery.NewDocument(rootUrl + "edit_bug.aspx?id=" + strconv.Itoa(bugId))
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
		commentMetadata = stripNonBreakingSpaces(commentMetadata)
		splitMetadata := strings.Split(commentMetadata, " ")
		
		// Check if the post contains any attachments.
		var attachment Attachment
		if splitMetadata[0] == "file" {
			attachmentInfo := commentData.Find(".pst")
			attachmentNameNode := commentData.Find("img").Parent().Next()
			// For now, ignore whether the href exists or not.
			attachmentUrl, _ := attachmentNameNode.Next().Attr("href")
			attachment = Attachment {
				name : attachmentNameNode.Text(),
				size : parseInt(strings.Split(stripNonBreakingSpaces(attachmentInfo.Last().Text()), " ")[1]),
				url: rootUrl + attachmentUrl,
			}
		}
		
		// There is one comment(!) on one bug which is different to all other
		// comments on all other bugs. The date reported for this comment
		// is just yyyy-m-d (e.g. no time component). To work around this,
		// check if the word which would normally hold the date actually
		// contains a colon ":". If it doesn't contain a colon, this must be
		// the special comment. 😡
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
			attachment: attachment,
		}
		
		// Skip this particular comment...
		if !isBlackListed(comment.id) {
			// Prepend the comment to the list of comments.
			comments = append([]Comment { comment }, comments...)
		}
	})
	return
}

// Fetches bug information from the bug tracker website.
// verbosity: level of verbosity. Currently we only check if this is > 0.
// n: Max number of bugs to fetch. Negative for unlimited.
// url: Root URL of the bug tracker website.
// Must contain trailing forward slash.
// e.g. https://www.apsim.info/BugTracker/
func getBugs(verbosity, n int, url string) (bugs []Bug) {
	if verbosity > 0 {
		fmt.Print("Downloading data...")
	}
	doc := loadBugList(url)
	if verbosity > 0 {
		fmt.Println("Finished!")
	}
	
	bugRows := doc.Find("table.bugt tr")
	numBugs := bugRows.IndexOfSelection(bugRows.Last())
	if n > 0 && n < numBugs {
		numBugs = n
	}
	
	bugRows.Each(func(index int, row *goquery.Selection) {
		if verbosity > 0 {
			fmt.Printf("Processing bugs...%.2f%%\r", float64(index) / float64(numBugs) * 100.0)
		}
		// Skip the first row of the table, as it doesn't contain bugs.
		if index > 0 && index < numBugs {
			bugId := parseInt(row.Find("td:nth-child(1)").Text())
			bugDate, err := time.Parse(dateFormat , row.Find("td:nth-child(8)").Text())
			if err != nil {
				fmt.Printf("Error parsing date in bug #%d\n", bugId)
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
				comments : getComments(url, int(bugId)),
			}
			bugs = append([]Bug { bug }, bugs...)
		}
	})
	if verbosity > 0 {
		fmt.Printf("Processing bugs...Finished!\n")
	}
	return
}

// Reads credentials from a text file.
func getCredentials() (username string, password string) {
	credentials, err := ioutil.ReadFile("credentials.txt")
	
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(credentials)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "username=") {
			username = strings.TrimPrefix(line, "username=")
		}
		if strings.HasPrefix(line, "password=") {
			password = strings.TrimPrefix(line, "password=")
		}
	}
	return
}

// Uploads a file to a remote server via FTP.
// remote: IP or hostname of the server.
// port: Port number (probably 21).
// webRoot: Root web directory.
// remoteDir: Directory to which the file will be copied. Relative to webRoot.
// localFile: path to the local file to be copied.
// user: username for the server.
// pass: password for the server.
// Returns the address of the remote file.
func uploadFileFtp(remote, port, webRoot, remoteDir, localFile, user, pass string) (string, error) {
	conn, err := ftp.DialTimeout(remote + ":" + port, 5 * time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Quit()
	
	err = conn.Login(user, pass)
	if err != nil {
		return "", err
	}
	
	err = conn.ChangeDir(webRoot)
	if err != nil {
		log.Fatal(err)
	}
	
	// This will return an error if the directory already exists.
	_ = conn.MakeDir(remoteDir)
	
	file, err := os.Open(localFile)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	remotePath := path.Join(remoteDir, filepath.Base(localFile))
	err = conn.Stor(remotePath, file)
	if err != nil {
		return "", err
	}
	
	return strings.Trim(remote, "/") + "/" + remotePath, nil
}

// Pauses execution for a random duration.
// min - minimum duration (in seconds).
// max - maximum duration (in seconds).
func randomSleep(min, max int) {
	ms := min * 1000 + rand.Intn((max - min) * 1000)
	time.Sleep(time.Duration(ms * int(time.Millisecond)))
}

// Posts a bug on GitHub
// bug: The bug to be posted on GitHub.
// org: Name of the organisation/owner of the repo.
// repo: Name of the GitHub repo to which the bug will be posted.
// credFile: Path to file on disk containing an access token for a GitHub account.
// reupload: If true, attachments will be downloaded from BugTracker, and uploaded to another site.
func postBug(bug Bug, org, repo, credFile string, reupload bool) {
	auth := octokit.TokenAuth{AccessToken: getSecret(credFile)}
	client := octokit.NewClient(auth)
	params := octokit.IssueParams {
		Title: bug.description,
		Body: bug.ToString(),
	}
	issue, result := client.Issues().Create(nil, octokit.M{"owner": "APSIMInitiative", "repo": "APSIMClassic"}, params)
	
	for result.HasError() {
		fmt.Printf("Encountered an error when attempting to post bug #%d\n", bug.id)
		fmt.Printf("result.Error(): \"%v\"\n", result.Error())
		if strings.Contains(result.Error(), "You have triggered an abuse detection mechanism and have been temporarily blocked from content creation.") {
			fmt.Printf("Triggered abuse detection mechanism on bug ID %d\n", bug.id)
			randomSleep(60, 120)
			issue, result = client.Issues().Create(nil, octokit.M{"owner": "APSIMInitiative", "repo": "APSIMClassic"}, params)
		} else {
			log.Fatal(result)
		}
	}
	if result.RateLimitRemaining() < 10 {
		time.Sleep(time.Hour)
	}
	tempDir := path.Join(os.TempDir(), "TransferIssues")
	CreateDirIfNotExist(tempDir)
	host := "www.apsim.info"
	port := "21"
	webRoot := "APSIM"
	for i, comment := range bug.comments {
		if i > 0 { // temporary hack
			// Deal with any attachments.
			if comment.attachment != (Attachment{}) {
				remoteDir := "BugAttachments/" + strconv.Itoa(int(comment.id))
				bug.comments[i].attachment.url = strings.Trim(host, "/") + "/" + remoteDir + "/" + bug.comments[i].attachment.GetCleanFileName()
				if reupload {
					localFile, err := comment.attachment.Download(tempDir)
					if err != nil {
						fmt.Printf("Error downloading file %v for bug #%d!\n", comment.attachment.name, bug.id)
						log.Fatal(err)
					}
					user, pass := getCredentials()
					
					bug.comments[i].attachment.url, err = uploadFileFtp(host, port, webRoot, remoteDir, localFile, user, pass)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
			input := octokit.M{"body": bug.comments[i].ToString()}
			_, result := client.IssueComments().Create(nil, octokit.M{"owner": "APSIMInitiative", "repo": "APSIMClassic", "number": issue.Number}, input)
			for result.HasError() {
				fmt.Printf("Encountered an error when attempting to post comment #%d\n", comment.id)
				fmt.Printf("result.Error(): \"%v\"\n", result.Error())
				if strings.Contains(result.Error(), "You have triggered an abuse detection mechanism and have been temporarily blocked from content creation.") {
					randomSleep(60, 120)
					_, result = client.IssueComments().Create(nil, octokit.M{"owner": "APSIMInitiative", "repo": "APSIMClassic", "number": issue.Number}, input)
				} else {
					log.Fatal(result)
				}
			}
			if result.RateLimitRemaining() < 10 {
				time.Sleep(time.Hour)
			} else {
				randomSleep(5, 10)
			}
		}
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	rootUrl := "https://www.apsim.info/BugTracker/"
	verbosity := 1
	maxBugs := -1
	doupload := false
	// Process command line arguments.
	for i := 0; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-q" || arg == "--quiet" {
			verbosity--
		} else if arg == "-v" || arg == "--verbose" {
			verbosity++
		} else if arg == "-n" {
			if i + 1 < len(os.Args) {
				i++
				maxBugs = int(parseInt(os.Args[i])) + 1
			} else {
				log.Fatal(fmt.Sprintf("Error: %v argument provided, but no value provided!", arg))
			}
		} else if arg == "-u" || arg == "--url" {
			if i + 1 < len(os.Args) {
				i++
				rootUrl = arg
			} else {
				log.Fatal(fmt.Sprintf("Error: %v argument provided, but no value provided!", arg))
			}
		} else if arg == "--reupload" {
			doupload = true
		}
	}
	
	// Get list of bugs.
	bugs := getBugs(verbosity, maxBugs, rootUrl)
	for i, bug := range bugs {
		if verbosity > 0 {
			fmt.Printf("Posting bugs...%.2f%%\r", float64(i) / float64(len(bugs)) * 100.0)
		}
		// Force attachments to be redownloaded/uploaded by setting the final
		// paramter to true.
		if bug.id > 1472 { // resume from where we left off
			postBug(bug, "APSIMInitiative", "APSIMClassic", "secret.txt", doupload)
			// Wait 10 seconds between posting each bug to avoid triggering
			// an API abuse error.
			randomSleep(5, 10)
		}
	}
	fmt.Println("Uploading attachments...Finished!")
	if verbosity > 1 {
		
		for _, bug := range bugs {
			fmt.Printf("%s\n", bug.description)
			fmt.Println(bug.ToString())
			fmt.Println("--------------------------------------------")
		}
	}
}