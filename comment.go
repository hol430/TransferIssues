package main

import (
	"fmt"
	"strings"
	"time"
)

type Comment struct {
	id				int64
	author			string
	date			time.Time
	text			string
	attachments		[]Attachment
}

func (c *Comment) ToString() string {
	var str strings.Builder
	
	str.WriteString(fmt.Sprintf("Comment ID: #%d\n", c.id))
	str.WriteString(fmt.Sprintf("Author: %v\n", c.author))
	str.WriteString(fmt.Sprintf("Date: %v\n", c.date))
	str.WriteString(fmt.Sprintf("Text: %v\n", c.text))
	for _, a := range c.attachments {
		str.WriteString(a.ToString())
	}
	return str.String()
}