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
	reporter		string
	date			time.Time
	assignee 		string
	comments		[]Comment
}

func (b *Bug) ToString() string {
	var str strings.Builder
	
	str.WriteString(fmt.Sprintf("Bug #%d\n", b.id))
	str.WriteString(fmt.Sprintf("Description: %v\n", b.description))
	str.WriteString(fmt.Sprintf("Priority: %v\n", b.priority))
	str.WriteString(fmt.Sprintf("Status: %v\n", b.status))
	str.WriteString(fmt.Sprintf("Project: %v\n", b.project))
	str.WriteString(fmt.Sprintf("Category: %v\n", b.category))
	str.WriteString(fmt.Sprintf("Reporter: %v\n", b.reporter))
	str.WriteString(fmt.Sprintf("Date: %v\n", b.date))
	str.WriteString(fmt.Sprintf("Assignee: %v\n", b.assignee))
	str.WriteString(fmt.Sprintf("Comments: %d\n", len(b.comments)))
	for _, comment := range b.comments {
		str.WriteString(comment.ToString())
	}
	return str.String()
}