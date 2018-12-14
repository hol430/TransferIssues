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
	attachment		Attachment
}

func (c *Comment) ToString() string {
	var str strings.Builder
	
	str.WriteString(fmt.Sprintf("Author: %v\n", c.author))
	str.WriteString(fmt.Sprintf("Date: %v\n\n", c.date))
	if c.attachment == (Attachment{}) {
		str.WriteString(c.text)
	} else {
		str.WriteString(c.attachment.ToString())
	}
	return str.String()
}