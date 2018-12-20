package main

import (
	"fmt"
	"strings"
	"time"
)

type Bug struct {
	id 				int64
	description 	string
	priority 		string
	status			string
	project			string
	category		string
	author			string
	date			time.Time
	assignee 		string
	comments		[]Comment
}

func (b *Bug) ToString() string {
	var str strings.Builder
	
	str.WriteString(fmt.Sprintf("Legacy Bug ID: %d\n", b.id))
	str.WriteString(fmt.Sprintf("Author: %v\n", b.author))
	str.WriteString(fmt.Sprintf("Date: %v\n", b.date))
	//str.WriteString(fmt.Sprintf("Title: %v\n\n", b.description))
	str.WriteString(fmt.Sprintf("Status: %s\n", b.status))
	if len(b.comments) > 0 {
		// The first comment on the bug tracker site contains the actual description of the bug.
		str.WriteString(fmt.Sprintf("\n%v\n", b.comments[0].text))
	}
	return str.String()
}

func (b *Bug) ToLongString() string {
	var str strings.Builder
	
	str.WriteString(b.ToString())
	for i, comment := range b.comments {
		if i > 0 {
			str.WriteString(fmt.Sprintf("\n\nCOMMENT %d:\n\n", i))
			str.WriteString(comment.ToString())
		} else {
			str.WriteString(comment.text)
		}
	}
	return str.String()
}

func (b *Bug) IsClosed() bool {
	return strings.ToLower(b.status) == "closed"
}