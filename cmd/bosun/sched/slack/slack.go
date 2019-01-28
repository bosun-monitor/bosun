package slack

import "bosun.org/models"

// NewAttachment returns a Attachment structure with the color
// and Timestamp set from the IncidentState.
func NewAttachment(m *models.IncidentState) *Attachment {
	a := new(Attachment)
	a.Color = StatusColor(m.CurrentStatus)
	a.Ts = m.LastAbnormalTime.Unix()
	return a
}

// StatusColor returns a color string that corresponds to Slack's colors
// based on the Status.
func StatusColor(status models.Status) string {
	switch status {
	case models.StNormal:
		return "good"
	case models.StWarning:
		return "warning"
	case models.StCritical:
		return "danger"
	case models.StUnknown:
		return "#439FE0"
	}
	return ""
}

// Attachment contains all the information for a slack message attachment and
// has methods to Set/Append fields.
type Attachment struct {
	Color    string `json:"color,omitempty"`
	Fallback string `json:"fallback"`

	AuthorID      string `json:"author_id,omitempty"`
	AuthorName    string `json:"author_name,omitempty"`
	AuthorSubname string `json:"author_subname,omitempty"`
	AuthorLink    string `json:"author_link,omitempty"`
	AuthorIcon    string `json:"author_icon,omitempty"`

	Title     string `json:"title,omitempty"`
	TitleLink string `json:"title_link,omitempty"`
	Pretext   string `json:"pretext,omitempty"`
	Text      string `json:"text"`

	ImageURL string `json:"image_url,omitempty"`
	ThumbURL string `json:"thumb_url,omitempty"`

	Fields  []interface{} `json:"fields,omitempty"`
	Actions []interface{} `json:"actions,omitempty"`

	Footer     string `json:"footer,omitempty"`
	FooterIcon string `json:"footer_icon,omitempty"`

	Ts int64 `json:"ts,omitempty"`
}

// AddActions appends an Action to the Actions field of the Attachment.
func (a *Attachment) AddActions(action ...interface{}) interface{} {
	a.Actions = append(a.Actions, action...)
	return "" // have to return something
}

// AddFields appends a Field to the Fields field of the Attachment.
func (a *Attachment) AddFields(field ...interface{}) interface{} {
	a.Fields = append(a.Fields, field...)
	return "" // have to return something
}

// SetColor appends an Action to the Actions field of the Attachment.
func (a *Attachment) SetColor(color string) interface{} {
	a.Color = color
	return ""
}

// SetFallback sets the FallBack field of the Attachment.
func (a *Attachment) SetFallback(fallback string) interface{} {
	a.Fallback = fallback
	return ""
}

// SetAuthorID sets the AuthorID field of the Attachment.
func (a *Attachment) SetAuthorID(authorID string) interface{} {
	a.AuthorID = authorID
	return ""
}

// SetAuthorName sets the AuthorName field of the Attachment.
func (a *Attachment) SetAuthorName(authorName string) interface{} {
	a.AuthorName = authorName
	return ""
}

// SetAuthorSubname sets the AuthorSubname field of the Attachment.
func (a *Attachment) SetAuthorSubname(authorSubname string) interface{} {
	a.AuthorSubname = authorSubname
	return ""
}

// SetAuthorLink sets the AuthorLink field of the Attachment.
func (a *Attachment) SetAuthorLink(authorLink string) interface{} {
	a.AuthorLink = authorLink
	return ""
}

// SetAuthorIcon sets the AuthorIcon field of the Attachment.
func (a *Attachment) SetAuthorIcon(authorIcon string) interface{} {
	a.AuthorIcon = authorIcon
	return ""
}

// SetTitle sets the Title field of the Attachment.
func (a *Attachment) SetTitle(title string) interface{} {
	a.Title = title
	return ""
}

// SetTitleLink sets the TitleLink field of the Attachment.
func (a *Attachment) SetTitleLink(titleLink string) interface{} {
	a.TitleLink = titleLink
	return ""
}

// SetPretext sets the Pretext field of the Attachment.
func (a *Attachment) SetPretext(pretext string) interface{} {
	a.Pretext = pretext
	return ""
}

// SetText sets the Text field of the Attachment.
func (a *Attachment) SetText(text string) interface{} {
	a.Text = text
	return ""
}

// SetImageURL sets the ImageURL field of the Attachment.
func (a *Attachment) SetImageURL(url string) interface{} {
	a.ImageURL = url
	return ""
}

// SetThumbURL sets the ThumbURL field of the Attachment.
func (a *Attachment) SetThumbURL(url string) interface{} {
	a.ThumbURL = url
	return ""
}

// SetFooter sets the Footer field of the Attachment.
func (a *Attachment) SetFooter(footer string) interface{} {
	a.Footer = footer
	return ""
}

// SetFooterIcon sets the FooterIcon field of the Attachment.
func (a *Attachment) SetFooterIcon(footerIcon string) interface{} {
	a.FooterIcon = footerIcon
	return ""
}

// SetTs sets the Ts field of the Attachment.
func (a *Attachment) SetTs(ts int64) interface{} {
	a.Ts = ts
	return ""
}
