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
	
	str.WriteString(fmt.Sprintf("Author: %v\n", c.author))
	str.WriteString(fmt.Sprintf("Date: %v\n\n", c.date))
	str.WriteString(c.text)
	for _, a := range c.attachments {
		str.WriteString("\n\n" + a.ToString() + "\n\n")
	}
	return str.String()
}