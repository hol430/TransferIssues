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
	"regexp"
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
			if bugId > 2000 {
				return
			}
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
	if bug.IsClosed() {
		closeIssue(org, repo, credFile, issue.Number)
	}
}

func getGithubIssues(owner, repo, credFile string, max int) (issues []octokit.Issue) {
	// Initialise github client
	auth := octokit.TokenAuth{AccessToken: getSecret(credFile)}
	client := octokit.NewClient(auth)
	url := octokit.Hyperlink(fmt.Sprintf("repos/%s/%s/issues", owner, repo))
	
	// A few state variables
	first := true
	numIssues := 0
	var progress float64
	for {
		bugs, result := client.Issues().All(&url, nil)
		if result.HasError() {
			log.Fatal(result)
		}
		
		if first {
			// First time through we record the issue number.
			// This is used to calculate progress each iteration.
			first = false
			if len(bugs) > 0 {
				numIssues = bugs[0].Number
				if max < 0 {
					max = numIssues
				}
			}
		}
		if len(bugs) > 0 {
			progress = 100.0 * float64(numIssues - bugs[0].Number) / float64(numIssues)
		}
		fmt.Printf("Fetching GitHub issues: %.2f%%\r", progress)
		issues = append(issues, bugs...)
		if result.NextPage == nil || len(issues) >= max{
			break
		}
		url = *result.NextPage
		if result.RateLimitRemaining() < 5 {
			fmt.Println("Rate limit exceeded. Sleeping for one hour...")
			time.Sleep(time.Hour)
			fmt.Println("One hour has elapsed. Resuming execution...")
		}
	}
	fmt.Printf("Fetching GitHub issues...Finished!\n")
	return
}

// I forgot to put https:// in front of attachment links. This function goes
// through all issues in the repository and fixes this mistake.
func fixLinks(credFile string, verbosity int) {
	re := regexp.MustCompile(`(\[[^\]]+\])\(www.apsim.info`)
	replaceRegex := "$1(https://www.apsim.info"
	auth := octokit.TokenAuth{AccessToken: getSecret(credFile)}
	client := octokit.NewClient(auth)
	apsimUrl := octokit.Hyperlink("repos/APSIMInitiative/APSIMClassic/issues")
	first := true
	numIssues := -1
	var progress float64
	for {
		issues, result := client.Issues().All(&apsimUrl, nil)
		if result.HasError() {
			log.Fatal(result)
		}
		for _, issue := range issues {
			if first {
				first = false
				numIssues = issue.Number
			}
			progress = 100.0 * float64(numIssues - issue.Number) / float64(numIssues)
			fmt.Printf("Fixing links...%.2f%%\r", progress)
			comments, result := client.IssueComments().All(&octokit.IssueCommentsURL, octokit.M{"owner": "APSIMInitiative", "repo": "APSIMClassic", "number": issue.Number})
			if result.HasError() {
				log.Fatal(result)
			}
			for _, comment := range comments {
				if re.MatchString(comment.Body) {
					if verbosity > 1 {
						fmt.Printf("Updating comment %d on bug %d\n", comment.ID, issue.Number)
					}
					comment.Body = re.ReplaceAllString(comment.Body, replaceRegex)
					fmt.Printf("Replacing comment on bug %d with:\n%s\n", issue.Number, comment.Body)
					input := octokit.M{"body": comment.Body}
					m := octokit.M{"owner": "APSIMInitiative", "repo": "APSIMClassic", "id": comment.ID}
					_, result = client.IssueComments().Update(nil, m, input)
					if result.HasError() {
						log.Fatal(result)
					} else if result.RateLimitRemaining() < 10 {
						time.Sleep(time.Hour)
					}
				}
			}
		}
		if result.NextPage == nil {
			break
		}
		apsimUrl = *result.NextPage
	}
	fmt.Printf("Fixing links...Finished!")
}

func fixLinksv2(credFile, rootUrl string, verbosity, n int) {
	bugs := getBugs(verbosity, n, rootUrl)
	re := regexp.MustCompile(`\[([^\]]+)\]\(https://www.apsim.info/BugTracker[^\)]+\)`)
	
	auth := octokit.TokenAuth{AccessToken: getSecret(credFile)}
	client := octokit.NewClient(auth)
	apsimUrl := octokit.Hyperlink("repos/APSIMInitiative/APSIMClassic/issues")
	first := true
	numIssues := -1
	var progress float64
	for {
		issues, result := client.Issues().All(&apsimUrl, nil)
		if result.HasError() {
			log.Fatal(result)
		}
		for _, issue := range issues {
			if first {
				first = false
				numIssues = issue.Number
			}
			progress = 100.0 * float64(numIssues - issue.Number) / float64(numIssues)
			fmt.Printf("Fixing links...%.2f%%\r", progress)
			comments, result := client.IssueComments().All(&octokit.IssueCommentsURL, octokit.M{"owner": "APSIMInitiative", "repo": "APSIMClassic", "number": issue.Number})
			if result.HasError() {
				log.Fatal(result)
			}
			for i, comment := range comments {
				if re.MatchString(comment.Body) {
					if verbosity > 1 {
						fmt.Printf("Match found for comment %d on bug %d\n", i + 1, issue.Number)
					}
					matches := re.FindStringSubmatch(comment.Body)
					if len(matches) >= 2 {
						legacyId := getLegacyId(issue)
						var bug Bug
						if legacyId >= 0 {
							bug = getBugFromId(bugs, legacyId)
						} else {
							bug = getBugFromTitle(bugs, issue.Title)
						}
						legacyComment := getCommentWithContent(bug.comments, matches[1])
						
						attachmentName := strings.Replace(matches[1], " ", "_", -1)
						replaceRegex := fmt.Sprintf("[$1](https://www.apsim.info/BugAttachments/%d/%s)", legacyComment.id, attachmentName)
						newBody := re.ReplaceAllString(comment.Body, replaceRegex)
						
						input := octokit.M{"body": newBody}
						m := octokit.M{"owner": "APSIMInitiative", "repo": "APSIMClassic", "id": comment.ID}
						_, result = client.IssueComments().Update(nil, m, input)
						if result.HasError() {
							log.Fatal(result)
						} else if result.RateLimitRemaining() < 10 {
							time.Sleep(time.Hour)
						}
					} else if verbosity > 1 {
						fmt.Printf("Number of matches: %d\n", len(matches))
					}
				} else if verbosity > 1 {
						fmt.Printf("No match found for comment %d on bug #%d\n", i + 1, issue.Number)
				}
			}
		}
		if result.NextPage == nil {
			break
		}
		apsimUrl = *result.NextPage
	}
	fmt.Printf("Fixing links...Finished!")
}

func getCommentWithContent(comments []Comment, content string) Comment {
	for _, comment := range comments {
		if strings.Contains(comment.text, content) {
			return comment
		}
	}
	for _, comment := range comments {
		fmt.Printf("Comment: %s\n", comment.text)
	}
	panic(fmt.Sprintf("Unable to get comment with a content %s.", content))
}

func getLegacyId(issue octokit.Issue) int {
	// This is the syntax which will be used in most issues.
	re := regexp.MustCompile(`Legacy Bug ID: (\d+)`)
	matches := re.FindStringSubmatch(issue.Body)
	if len(matches) >= 2 {
		id, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Fatal(err)
		}
		return id
	}
	
	// Older versions of this program used this syntax.
	re = regexp.MustCompile(`Bug #(\d+)`)
	matches = re.FindStringSubmatch(issue.Body)
	if len(matches) >= 2 {
		id, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Fatal(err)
		}
		return id
	}
	fmt.Printf("Warning: Unable to determine legacy bug ID for GitHub Issue #%d\n", issue.Number)
	fmt.Printf("Resorting to title match.\n")
	return -1
}

// Finds the bug with the given ID.
// bugs: list of bugs.
// id: ID of the bug.
func getBugFromId(bugs []Bug, id int) Bug {
	for _, issue := range bugs {
		if issue.id == int64(id) {
			return issue
		}
	}
	panic(fmt.Sprintf("Unable to find bug with ID %d\n", id))
}

// Finds the bug with the given title.
// bugs: list of bugs.
// title: Title of the bug.
func getBugFromTitle(bugs []Bug, title string) Bug {
	for _, bug := range bugs {
		if bug.description == title {
			return bug
		}
	}
	panic(fmt.Sprintf("Unable to find bug with title %s\n", title))
}
// Closes a GitHub issue
// owner: Owner of the repo.
// repo: Name of the repo.
// credFile: path to a file on disk containing a github personal access token.
// id: ID of the issue to close.
func closeIssue(owner, repo, credFile string, id int) {
	auth := octokit.TokenAuth{AccessToken: getSecret(credFile)}
	client := octokit.NewClient(auth)
	m := octokit.M{"owner": owner, "repo": repo, "number": id}
	params := octokit.IssueParams{State: "closed"}
	_, result := client.Issues().Update(nil, m, params)
	if result.HasError() {
		log.Fatal(result)
	} else if result.RateLimitRemaining() < 5 {
		fmt.Println("GitHub API rate limit exceeded. Waiting for one hour...")
		time.Sleep(time.Hour)
		fmt.Println("One hour has elapsed. Resuming execution...")
	}
}

// Fetches bugs from bug tracker site and for those which are closed,
// closes their github counterpart.
// credFile: credentials file containing a github personal access token.
// verbosity: level of output detail
func closeIssues(rootUrl, credFile string, verbosity, maxBugs int) {
	owner := "APSIMInitiative"
	repo := "APSIMClassic"
	issues := getGithubIssues(owner, repo, credFile, maxBugs)
	bugTrackerIssues := getBugs(verbosity, maxBugs, rootUrl)
	for _, issue := range issues {
		legacyId := getLegacyId(issue)
		var legacyIssue Bug
		if legacyId >= 0 {
			legacyIssue = getBugFromId(bugTrackerIssues, legacyId)	
		} else {
			legacyIssue = getBugFromTitle(bugTrackerIssues, issue.Title)
		}
		if legacyIssue.IsClosed() && strings.ToLower(issue.State) != "closed" {
			fmt.Printf("Closing issue %d (#%d - %s)\n", legacyId, issue.Number, issue.State)
			closeIssue(owner, repo, credFile, issue.Number)
		} else {
			fmt.Printf("Skipping issue %d (#%d - %s)\n", legacyId, issue.Number, issue.State)
		}
	}
}

// Fixes formatting of bugs which incorrectly have tabs inserted in them.
func fixFormatting(credFile string, verbosity, n int) {
	auth := octokit.TokenAuth{AccessToken: getSecret(credFile)}
	client := octokit.NewClient(auth)
	var m octokit.M
	owner := "APSIMInitiative"
	repo := "APSIMClassic"
	issues := getGithubIssues(owner, repo, credFile, n)
	numIssues := len(issues)
	var progress float64
	for i, issue := range issues {
		progress = 100.0 * float64(i) / float64(numIssues)
		fmt.Printf("Fixing formatting: %.2f%%\r", progress)
		if strings.Contains(issue.Body, "\t") {
			newBody := strings.Replace(issue.Body, "\t", "", -1)
			if verbosity > 1 {
				fmt.Printf("Replacing tabs in Bug #%d\n", issue.Number)
				if verbosity > 2 {
					fmt.Printf("\"%s\"\n", issue.Body)
					fmt.Printf("\"%s\"\n", newBody)
					fmt.Println("------------------------------------------------")
				}
			}
			m = octokit.M{"owner": owner, "repo": repo, "number": issue.Number}
			params := octokit.IssueParams {Body: newBody}
			_, result := client.Issues().Update(nil, m, params)
			if result.HasError() {
				log.Fatal(result)
			} else if result.RateLimitRemaining() < 5 {
				fmt.Println("GitHub API rate limit exceeded. Waiting for one hour...")
				time.Sleep(time.Hour)
				fmt.Println("One hour has elapsed. Resuming execution...")
			}
		}
		
		// Fix formatting for comments on this issue
		comments, result := client.IssueComments().All(&octokit.IssueCommentsURL, octokit.M{"owner": owner, "repo": repo, "number": issue.Number})
		if result.HasError() {
			log.Fatal(result)
		}
		for commentNo, comment := range comments {
			if strings.Contains(comment.Body, "\t") {
				if verbosity > 1 {
					fmt.Printf("Updating comment %d of issue #%d\n", commentNo + 1, issue.Number)
				}
				newCommentBody := strings.Replace(comment.Body, "\t", "", -1)
				m = octokit.M{"owner": owner, "repo": repo, "id": comment.ID}
				params := octokit.M{"body": newCommentBody}
				_, result = client.IssueComments().Update(nil, m, params)
				if result.HasError() {
					log.Fatal(result)
				} else if result.RateLimitRemaining() < 5 {
					fmt.Println("GitHub API rate limit exceeded. Waiting for one hour...")
					time.Sleep(time.Hour)
					fmt.Println("One hour has elapsed. Resuming execution...")
				}
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
	fixlinks := false
	closeissues := false
	fixformatting := false
	fixlinks2 := false
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
		} else if arg == "--fix-links" {
			fixlinks = true
		} else if arg == "--close-issues" {
			closeissues = true
		} else if arg == "--fix-formatting" {
			fixformatting = true
		} else if arg == "--fix-links2" {
			fixlinks2 = true
		}
	}
	if fixlinks {
		fixLinks("secret.txt", verbosity)
	} else if fixlinks2 {
		fixLinksv2("secret.txt", rootUrl, verbosity, maxBugs)
	} else if closeissues {
		closeIssues(rootUrl, "secret.txt", verbosity, maxBugs)
	} else if fixformatting {
		fixFormatting("secret.txt", verbosity, maxBugs)
	}else {
		fmt.Printf("doupload=%v\n", doupload)
		// Get list of bugs.
		bugs := getBugs(verbosity, maxBugs, rootUrl)
		for i, bug := range bugs {
			if verbosity > 0 {
				fmt.Printf("Posting bugs...%.2f%%\r", float64(i) / float64(len(bugs)) * 100.0)
			}
			// Force attachments to be redownloaded/uploaded by setting the final
			// paramter to true.
			if bug.id >= 0 { // Use this to resume from a failed/aborted execution.
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
}